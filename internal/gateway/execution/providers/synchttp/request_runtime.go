package synchttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformgeneral "github.com/sh2001sh/CodeGo-Api/internal/platform/general"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type RequestAdaptor interface {
	GetRequestURL(info *relaycommon.RelayInfo) (string, error)
	SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
}

func SetupAPIRequestHeader(info *relaycommon.RelayInfo, c *gin.Context, req *http.Header) {
	if info.RelayMode == gatewaycontract.RelayModeAudioTranscription || info.RelayMode == gatewaycontract.RelayModeAudioTranslation {
		return
	}
	if info.RelayMode == gatewaycontract.RelayModeRealtime {
		return
	}
	req.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Set("Accept", c.Request.Header.Get("Accept"))
	if info.IsStream && c.Request.Header.Get("Accept") == "" {
		req.Set("Accept", "text/event-stream")
	}
}

func applyUpstreamContentLength(req *http.Request, info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	if info.UpstreamRequestBodySize > 0 && req.ContentLength <= 0 {
		req.ContentLength = info.UpstreamRequestBodySize
	}
}

func startPingKeepAlive(c *gin.Context, pingInterval time.Duration) context.CancelFunc {
	pingerCtx, stopPinger := context.WithCancel(context.Background())

	gopool.Go(func() {
		defer func() {
			_ = recover()
		}()

		if pingInterval <= 0 {
			pingInterval = gatewaystream.DefaultPingInterval
		}

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		var pingMutex sync.Mutex
		pingTimeout := time.NewTimer(120 * time.Minute)
		defer pingTimeout.Stop()

		for {
			select {
			case <-ticker.C:
				if err := sendPingData(c, &pingMutex); err != nil {
					return
				}
			case <-pingerCtx.Done():
				return
			case <-c.Request.Context().Done():
				return
			case <-pingTimeout.C:
				return
			}
		}
	})

	return stopPinger
}

func sendPingData(c *gin.Context, mutex *sync.Mutex) error {
	done := make(chan error, 1)
	go func() {
		mutex.Lock()
		defer mutex.Unlock()

		err := gatewaystream.PingData(c)
		if err != nil {
			logger.LogError(c, "SSE ping error: "+err.Error())
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("SSE ping data send timeout")
	case <-c.Request.Context().Done():
		return errors.New("request context cancelled during ping")
	}
}

func DoRequest(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) (*http.Response, error) {
	var client *http.Client
	var err error
	if info.ChannelSetting.Proxy != "" {
		client, err = platformhttpx.NewProxyHTTPClient(info.ChannelSetting.Proxy)
		if err != nil {
			return nil, fmt.Errorf("new proxy http client failed: %w", err)
		}
	} else {
		client = platformhttpx.GetHTTPClient()
	}

	var stopPinger context.CancelFunc
	if info.IsStream {
		gatewaystream.SetEventStreamHeaders(c)
		generalSettings := platformgeneral.GetSetting()
		if generalSettings.PingIntervalEnabled && !info.DisablePing {
			pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
			stopPinger = startPingKeepAlive(c, pingInterval)
			defer func() {
				if stopPinger != nil {
					stopPinger()
				}
			}()
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(c, "do request failed: "+err.Error())
		if isUpstreamResponseTimeout(err) {
			return nil, types.NewError(
				err,
				types.ErrorCodeChannelResponseTimeExceeded,
				types.ErrOptionWithStatusCode(http.StatusGatewayTimeout),
				types.ErrOptionWithHideErrMsg("upstream response timed out"),
			)
		}
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream error: do request failed"))
	}
	if resp == nil {
		return nil, errors.New("resp is nil")
	}

	if upID := resp.Header.Get(constant.RequestIdKey); upID != "" {
		c.Set(constant.UpstreamRequestIdKey, upID)
	}

	_ = req.Body.Close()
	_ = c.Request.Body.Close()
	return resp, nil
}

func isUpstreamResponseTimeout(err error) bool {
	if err == nil {
		return false
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout awaiting response headers") || strings.Contains(message, "response header timeout")
}

func DoAPIRequest(a RequestAdaptor, c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	if platformconfig.DebugEnabled {
		println("fullRequestURL:", fullRequestURL)
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	applyUpstreamContentLength(req, info)

	headers := req.Header
	if err := a.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	headerOverride, err := resolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	applyHeaderOverrideToRequest(req, headerOverride)

	resp, err := DoRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func DoFormAPIRequest(a RequestAdaptor, c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	if platformconfig.DebugEnabled {
		println("fullRequestURL:", fullRequestURL)
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	applyUpstreamContentLength(req, info)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	headers := req.Header
	if err := a.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	headerOverride, err := resolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	applyHeaderOverrideToRequest(req, headerOverride)

	resp, err := DoRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func DoWSSRequest(a RequestAdaptor, c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*websocket.Conn, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}

	targetHeader := http.Header{}
	if err := a.SetupRequestHeader(c, &targetHeader, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	headerOverride, err := resolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for key, value := range headerOverride {
		targetHeader.Set(key, value)
	}
	targetHeader.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	targetConn, _, err := websocket.DefaultDialer.Dial(fullRequestURL, targetHeader)
	if err != nil {
		return nil, fmt.Errorf("dial failed to %s: %w", fullRequestURL, err)
	}
	return targetConn, nil
}
