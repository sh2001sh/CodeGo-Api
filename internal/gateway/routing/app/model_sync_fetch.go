package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
)

type upstreamEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    []T    `json:"data"`
}

type upstreamModel struct {
	Description string          `json:"description"`
	Endpoints   json.RawMessage `json:"endpoints"`
	Icon        string          `json:"icon"`
	ModelName   string          `json:"model_name"`
	NameRule    int             `json:"name_rule"`
	Status      int             `json:"status"`
	Tags        string          `json:"tags"`
	VendorName  string          `json:"vendor_name"`
}

type upstreamVendor struct {
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
}

var (
	etagCache  = make(map[string]string)
	bodyCache  = make(map[string][]byte)
	cacheMutex sync.RWMutex
)

var (
	httpClientOnce sync.Once
	httpClient     *http.Client
)

func normalizeLocale(locale string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(locale))
	switch value {
	case "en", "zh-cn", "zh-tw", "ja":
		return value, true
	default:
		return "", false
	}
}

func getUpstreamBase() string {
	return platformconfig.GetEnvOrDefaultString("SYNC_UPSTREAM_BASE", "https://basellm.github.io/llm-metadata")
}

func ResolveUpstreamSource(locale string) UpstreamSource {
	base := strings.TrimRight(getUpstreamBase(), "/")
	source := UpstreamSource{Locale: locale}
	if normalized, ok := normalizeLocale(locale); ok && normalized != "" {
		source.ModelsURL = fmt.Sprintf("%s/api/i18n/%s/codegoapi/models.json", base, normalized)
		source.VendorsURL = fmt.Sprintf("%s/api/i18n/%s/codegoapi/vendors.json", base, normalized)
		return source
	}
	source.ModelsURL = fmt.Sprintf("%s/api/codegoapi/models.json", base)
	source.VendorsURL = fmt.Sprintf("%s/api/codegoapi/vendors.json", base)
	return source
}

func newSyncHTTPClient() *http.Client {
	timeoutSec := platformconfig.GetEnvOrDefaultInt("SYNC_HTTP_TIMEOUT_SECONDS", 10)
	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}
	transport := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   time.Duration(timeoutSec) * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: time.Duration(timeoutSec) * time.Second,
	}
	if platformconfig.TLSInsecureSkipVerify {
		transport.TLSClientConfig = platformconfig.InsecureTLSConfig
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		if strings.HasSuffix(host, "github.io") {
			if conn, err := dialer.DialContext(ctx, "tcp4", addr); err == nil {
				return conn, nil
			}
			return dialer.DialContext(ctx, "tcp6", addr)
		}
		return dialer.DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: transport}
}

func getSyncHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = newSyncHTTPClient()
	})
	return httpClient
}

func fetchJSON[T any](ctx context.Context, url string, out *upstreamEnvelope[T]) error {
	attempts := platformconfig.GetEnvOrDefaultInt("SYNC_HTTP_RETRY", 3)
	if attempts < 1 {
		attempts = 1
	}
	maxMB := platformconfig.GetEnvOrDefaultInt("SYNC_HTTP_MAX_MB", 10)
	maxBytes := int64(maxMB) << 20
	baseDelay := 200 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		lastErr = fetchJSONOnce(ctx, url, maxBytes, out)
		if lastErr == nil {
			return nil
		}
		sleep := baseDelay * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Intn(150)) * time.Millisecond
		time.Sleep(sleep + jitter)
	}
	return lastErr
}

func fetchJSONOnce[T any](ctx context.Context, url string, maxBytes int64, out *upstreamEnvelope[T]) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	cacheMutex.RLock()
	if et := etagCache[url]; et != "" {
		req.Header.Set("If-None-Match", et)
	}
	cacheMutex.RUnlock()

	resp, err := getSyncHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		buf, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
		if err != nil {
			return err
		}
		cacheResponse(url, resp.Header.Get("ETag"), buf)
		return decodeEnvelope(buf, out)
	case http.StatusNotModified:
		buf := getCachedBody(url)
		if len(buf) == 0 {
			return errors.New("cache miss for 304 response")
		}
		return decodeEnvelope(buf, out)
	default:
		return errors.New(resp.Status)
	}
}

func cacheResponse(url string, etag string, body []byte) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if etag != "" {
		etagCache[url] = etag
	}
	bodyCache[url] = body
}

func getCachedBody(url string) []byte {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return bodyCache[url]
}

func decodeEnvelope[T any](buf []byte, out *upstreamEnvelope[T]) error {
	if err := json.Unmarshal(buf, out); err == nil {
		if !out.Success && len(out.Data) == 0 && out.Message == "" {
			out.Success = true
		}
		return nil
	}

	var arr []T
	if err := json.Unmarshal(buf, &arr); err != nil {
		return err
	}
	out.Success = true
	out.Data = arr
	out.Message = ""
	return nil
}

func fetchUpstreamData(ctx context.Context, source UpstreamSource) (map[string]upstreamVendor, map[string]upstreamModel, error) {
	var vendorsEnv upstreamEnvelope[upstreamVendor]
	var modelsEnv upstreamEnvelope[upstreamModel]
	var fetchErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = fetchJSON(ctx, source.VendorsURL, &vendorsEnv)
	}()
	go func() {
		defer wg.Done()
		if err := fetchJSON(ctx, source.ModelsURL, &modelsEnv); err != nil {
			fetchErr = err
		}
	}()
	wg.Wait()
	if fetchErr != nil {
		return nil, nil, &UpstreamFetchError{Source: source, Err: fetchErr}
	}

	vendorByName := make(map[string]upstreamVendor)
	for _, vendor := range vendorsEnv.Data {
		if vendor.Name != "" {
			vendorByName[vendor.Name] = vendor
		}
	}
	modelByName := make(map[string]upstreamModel)
	for _, item := range modelsEnv.Data {
		if item.ModelName != "" {
			modelByName[item.ModelName] = item
		}
	}
	return vendorByName, modelByName, nil
}
