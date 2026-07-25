package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FetchUpstreamRatios loads upstream pricing data and compares it against the local ratio config.

func FetchUpstreamRatios(ctx context.Context, req dto.UpstreamRequest) (*RatioSyncFetchResult, error) {
	if req.Timeout <= 0 {
		req.Timeout = defaultTimeoutSeconds
	}

	upstreams, err := collectRequestedUpstreams(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(upstreams) == 0 {
		return nil, ErrNoValidUpstreams
	}

	client := newRatioSyncHTTPClient()
	sem := make(chan struct{}, maxConcurrentFetches)
	resultsCh := make(chan ratioSyncUpstreamResult, len(upstreams))

	var wg sync.WaitGroup
	for _, upstream := range upstreams {
		wg.Add(1)
		go func(upstream dto.UpstreamDTO) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resultsCh <- fetchUpstreamRatioData(ctx, client, req.Timeout, upstream)
		}(upstream)
	}

	wg.Wait()
	close(resultsCh)

	localData := getLocalPricingSyncData()
	testResults := make([]dto.TestResult, 0, len(upstreams))
	successfulChannels := make([]ratioSyncChannelData, 0, len(upstreams))

	for result := range resultsCh {
		if result.Err != "" {
			testResults = append(testResults, dto.TestResult{
				Name:   result.Name,
				Status: "error",
				Error:  result.Err,
			})
			continue
		}
		testResults = append(testResults, dto.TestResult{
			Name:   result.Name,
			Status: "success",
		})
		successfulChannels = append(successfulChannels, ratioSyncChannelData{
			name: result.Name,
			data: result.Data,
		})
	}

	return &RatioSyncFetchResult{
		Differences: buildDifferences(localData, successfulChannels),
		TestResults: testResults,
	}, nil
}

func collectRequestedUpstreams(ctx context.Context, req dto.UpstreamRequest) ([]dto.UpstreamDTO, error) {
	upstreams := make([]dto.UpstreamDTO, 0)
	if len(req.Upstreams) > 0 {
		for _, upstream := range req.Upstreams {
			if !strings.HasPrefix(upstream.BaseURL, "http") {
				continue
			}
			if upstream.Endpoint == "" {
				upstream.Endpoint = defaultEndpoint
			}
			upstream.BaseURL = strings.TrimRight(upstream.BaseURL, "/")
			upstreams = append(upstreams, upstream)
		}
		return upstreams, nil
	}

	if len(req.ChannelIDs) == 0 {
		return upstreams, nil
	}

	channelIDs := make([]int, 0, len(req.ChannelIDs))
	for _, id := range req.ChannelIDs {
		channelIDs = append(channelIDs, int(id))
	}

	channels, err := gatewaystore.LoadChannelsByIDs(channelIDs)
	if err != nil {
		logger.LogError(ctx, "failed to query channels: "+err.Error())
		return nil, fmt.Errorf("%w: %v", ErrQueryChannelsFailed, err)
	}

	for _, channel := range channels {
		baseURL := channel.GetBaseURL()
		if !strings.HasPrefix(baseURL, "http") {
			continue
		}
		upstreams = append(upstreams, dto.UpstreamDTO{
			ID:       channel.Id,
			Name:     channel.Name,
			BaseURL:  strings.TrimRight(baseURL, "/"),
			Endpoint: "",
		})
	}
	return upstreams, nil
}

func newRatioSyncHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
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

func fetchUpstreamRatioData(ctx context.Context, client *http.Client, timeoutSeconds int, upstream dto.UpstreamDTO) ratioSyncUpstreamResult {
	isOpenRouter := upstream.Endpoint == "openrouter"
	fullURL := buildUpstreamRatioURL(upstream, isOpenRouter)
	isModelsDev := isModelsDevAPIEndpoint(fullURL)
	uniqueName := upstream.Name
	if upstream.ID != 0 {
		uniqueName = fmt.Sprintf("%s(%d)", upstream.Name, upstream.ID)
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, fullURL, nil)
	if err != nil {
		logger.LogWarn(ctx, "build request failed: "+err.Error())
		return ratioSyncUpstreamResult{Name: uniqueName, Err: err.Error()}
	}

	if err := applyOpenRouterAuthorization(request, upstream, isOpenRouter); err != nil {
		return ratioSyncUpstreamResult{Name: uniqueName, Err: err.Error()}
	}

	bodyBytes, err := executeRatioSyncRequest(ctx, client, request, upstream.Name)
	if err != nil {
		return ratioSyncUpstreamResult{Name: uniqueName, Err: err.Error()}
	}

	converted, err := decodeUpstreamRatioPayload(ctx, upstream.Name, bodyBytes, isOpenRouter, isModelsDev)
	if err != nil {
		return ratioSyncUpstreamResult{Name: uniqueName, Err: err.Error()}
	}
	return ratioSyncUpstreamResult{Name: uniqueName, Data: converted}
}

func buildUpstreamRatioURL(upstream dto.UpstreamDTO, isOpenRouter bool) string {
	endpoint := upstream.Endpoint
	if isOpenRouter {
		return upstream.BaseURL + "/v1/models"
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	} else if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return upstream.BaseURL + endpoint
}

func applyOpenRouterAuthorization(request *http.Request, upstream dto.UpstreamDTO, isOpenRouter bool) error {
	if !isOpenRouter {
		return nil
	}
	if upstream.ID == 0 {
		return fmt.Errorf("OpenRouter requires a valid channel with API key")
	}

	channel, err := gatewaystore.LoadChannelByID(upstream.ID, true)
	if err != nil {
		return fmt.Errorf("failed to get channel key: %w", err)
	}
	key, _, apiErr := gatewaystore.GetNextEnabledChannelKey(channel)
	if apiErr != nil {
		return fmt.Errorf("failed to get enabled channel key: %w", apiErr)
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("no API key configured for this channel")
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	return nil
}

func executeRatioSyncRequest(ctx context.Context, client *http.Client, request *http.Request, upstreamName string) ([]byte, error) {
	var response *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		response, lastErr = client.Do(request)
		if lastErr == nil {
			break
		}
		time.Sleep(time.Duration(200*(1<<attempt)) * time.Millisecond)
	}
	if lastErr != nil {
		logger.LogWarn(ctx, "http error on "+upstreamName+": "+lastErr.Error())
		return nil, lastErr
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.LogWarn(ctx, "non-200 from "+upstreamName+": "+response.Status)
		return nil, fmt.Errorf("%s", response.Status)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		logger.LogWarn(ctx, "unexpected content-type from "+upstreamName+": "+contentType)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, maxRatioConfigBytes))
	if err != nil {
		logger.LogWarn(ctx, "read response failed from "+upstreamName+": "+err.Error())
		return nil, err
	}
	return bodyBytes, nil
}

func decodeUpstreamRatioPayload(ctx context.Context, upstreamName string, bodyBytes []byte, isOpenRouter bool, isModelsDev bool) (map[string]any, error) {
	if isOpenRouter {
		converted, err := convertOpenRouterToRatioData(bytes.NewReader(bodyBytes))
		if err != nil {
			logger.LogWarn(ctx, "OpenRouter parse failed from "+upstreamName+": "+err.Error())
			return nil, err
		}
		return converted, nil
	}
	if isModelsDev {
		converted, err := convertModelsDevToRatioData(bytes.NewReader(bodyBytes))
		if err != nil {
			logger.LogWarn(ctx, "models.dev parse failed from "+upstreamName+": "+err.Error())
			return nil, err
		}
		return converted, nil
	}

	var body struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := platformencoding.DecodeJSON(bytes.NewReader(bodyBytes), &body); err != nil {
		logger.LogWarn(ctx, "json decode failed from "+upstreamName+": "+err.Error())
		return nil, err
	}
	if !body.Success {
		return nil, fmt.Errorf("%s", body.Message)
	}

	if type1Data, ok := decodeType1RatioData(body.Data); ok {
		return type1Data, nil
	}

	var items []pricingItem
	if err := platformencoding.Unmarshal(body.Data, &items); err != nil {
		logger.LogWarn(ctx, "unrecognized data format from "+upstreamName+": "+err.Error())
		return nil, fmt.Errorf("无法解析上游返回数据")
	}
	return convertPricingItemsToRatioData(items), nil
}

func decodeType1RatioData(raw json.RawMessage) (map[string]any, bool) {
	var type1Data map[string]any
	if err := platformencoding.Unmarshal(raw, &type1Data); err != nil {
		return nil, false
	}
	for _, field := range pricingSyncFields {
		if _, ok := type1Data[field]; ok {
			return type1Data, true
		}
	}
	return nil, false
}

func convertPricingItemsToRatioData(items []pricingItem) map[string]any {
	modelRatioMap := make(map[string]float64)
	completionRatioMap := make(map[string]float64)
	cacheRatioMap := make(map[string]float64)
	createCacheRatioMap := make(map[string]float64)
	imageRatioMap := make(map[string]float64)
	audioRatioMap := make(map[string]float64)
	audioCompletionRatioMap := make(map[string]float64)
	modelPriceMap := make(map[string]float64)
	billingModeMap := make(map[string]string)
	billingExprMap := make(map[string]string)

	for _, item := range items {
		if item.ModelName == "" {
			continue
		}
		if item.BillingMode == gatewaystore.BillingModeTieredExpr && strings.TrimSpace(item.BillingExpr) != "" {
			billingModeMap[item.ModelName] = gatewaystore.BillingModeTieredExpr
			billingExprMap[item.ModelName] = item.BillingExpr
		}
		if item.QuotaType == 1 {
			modelPriceMap[item.ModelName] = item.ModelPrice
		} else {
			modelRatioMap[item.ModelName] = item.ModelRatio
			completionRatioMap[item.ModelName] = item.CompletionRatio
		}
		if item.CacheRatio != nil {
			cacheRatioMap[item.ModelName] = *item.CacheRatio
		}
		if item.CreateCacheRatio != nil {
			createCacheRatioMap[item.ModelName] = *item.CreateCacheRatio
		}
		if item.ImageRatio != nil {
			imageRatioMap[item.ModelName] = *item.ImageRatio
		}
		if item.AudioRatio != nil {
			audioRatioMap[item.ModelName] = *item.AudioRatio
		}
		if item.AudioCompletionRatio != nil {
			audioCompletionRatioMap[item.ModelName] = *item.AudioCompletionRatio
		}
	}

	converted := make(map[string]any)
	if len(modelRatioMap) > 0 {
		ratioAny := make(map[string]any, len(modelRatioMap))
		for key, value := range modelRatioMap {
			ratioAny[key] = value
		}
		converted["model_ratio"] = ratioAny
	}
	if len(completionRatioMap) > 0 {
		completionAny := make(map[string]any, len(completionRatioMap))
		for key, value := range completionRatioMap {
			completionAny[key] = value
		}
		converted["completion_ratio"] = completionAny
	}
	if len(cacheRatioMap) > 0 {
		converted["cache_ratio"] = valueMap(cacheRatioMap)
	}
	if len(createCacheRatioMap) > 0 {
		converted["create_cache_ratio"] = valueMap(createCacheRatioMap)
	}
	if len(imageRatioMap) > 0 {
		converted["image_ratio"] = valueMap(imageRatioMap)
	}
	if len(audioRatioMap) > 0 {
		converted["audio_ratio"] = valueMap(audioRatioMap)
	}
	if len(audioCompletionRatioMap) > 0 {
		converted["audio_completion_ratio"] = valueMap(audioCompletionRatioMap)
	}
	if len(modelPriceMap) > 0 {
		priceAny := make(map[string]any, len(modelPriceMap))
		for key, value := range modelPriceMap {
			priceAny[key] = value
		}
		converted["model_price"] = priceAny
	}
	if len(billingModeMap) > 0 {
		converted[gatewaystore.BillingModeField] = valueMap(billingModeMap)
	}
	if len(billingExprMap) > 0 {
		converted[gatewaystore.BillingExprField] = valueMap(billingExprMap)
	}
	return converted
}
