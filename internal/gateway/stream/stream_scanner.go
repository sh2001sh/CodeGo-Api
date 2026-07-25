package stream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformconcurrency "github.com/sh2001sh/CodeGo-Api/internal/platform/concurrency"
	platformgeneral "github.com/sh2001sh/CodeGo-Api/internal/platform/general"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
)

const (
	InitialScannerBufferSize    = 64 << 10
	DefaultMaxScannerBufferSize = 64 << 20
	DefaultPingInterval         = 10 * time.Second
)

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func NewStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	return scanner
}

func ScanResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *Result)) {
	if resp == nil || dataHandler == nil {
		return
	}

	info.StreamStatus = gatewaycontract.NewStreamStatus()

	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	var (
		stopChan   = make(chan bool, 3)
		scanner    = NewStreamScanner(resp.Body)
		ticker     = time.NewTicker(streamingTimeout)
		pingTicker *time.Ticker
		writeMutex sync.Mutex
		wg         sync.WaitGroup
	)

	generalSettings := platformgeneral.GetSetting()
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}
	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	defer func() {
		platformconcurrency.SafeSendBool(stopChan, true)
		ticker.Stop()
		if pingTicker != nil {
			pingTicker.Stop()
		}

		done := make(chan struct{})
		gopool.Go(func() {
			wg.Wait()
			close(done)
		})

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logger.LogError(c, "timeout waiting for goroutines to exit")
		}

		close(stopChan)
	}()

	scanner.Split(bufio.ScanLines)
	SetEventStreamHeaders(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	if pingEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					platformconcurrency.SafeSendBool(stopChan, true)
				}
			}()

			pingTimeout := time.NewTimer(30 * time.Minute)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					done := make(chan error, 1)
					gopool.Go(func() {
						writeMutex.Lock()
						defer writeMutex.Unlock()
						done <- PingData(c)
					})

					select {
					case err := <-done:
						if err != nil {
							logger.LogError(c, "ping data error: "+err.Error())
							info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonPingFail, err)
							return
						}
					case <-time.After(10 * time.Second):
						logger.LogError(c, "ping data send timeout")
						info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonPingFail, fmt.Errorf("ping send timeout"))
						return
					case <-ctx.Done():
						return
					case <-stopChan:
						return
					}
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan string, 10)
	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			platformconcurrency.SafeSendBool(stopChan, true)
		}()
		sr := newResult(info.StreamStatus)
		for data := range dataChan {
			sr.reset()
			writeMutex.Lock()
			dataHandler(data, sr)
			writeMutex.Unlock()
			if sr.IsStopped() {
				return
			}
		}
	})

	wg.Add(1)
	platformconcurrency.RelayCtxGo(ctx, func() {
		defer func() {
			close(dataChan)
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			platformconcurrency.SafeSendBool(stopChan, true)
		}()

		for scanner.Scan() {
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonClientGone, c.Request.Context().Err())
				return
			default:
			}

			ticker.Reset(streamingTimeout)
			data := scanner.Text()
			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = strings.TrimSpace(data[5:])
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				info.SetFirstResponseTime()
				info.ReceivedResponseCount++
				select {
				case dataChan <- data:
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				}
			} else {
				info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonDone, nil)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				logger.LogError(c, "scanner error: "+err.Error())
				info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonScannerErr, err)
			}
		}
		info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonEOF, nil)
	})

	select {
	case <-ticker.C:
		info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonTimeout, nil)
	case <-stopChan:
	case <-c.Request.Context().Done():
		info.StreamStatus.SetEndReason(gatewaycontract.StreamEndReasonClientGone, c.Request.Context().Err())
	}

	if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
	} else {
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
	}
}
