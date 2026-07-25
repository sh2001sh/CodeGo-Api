package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	"github.com/sh2001sh/CodeGo-Api/internal/billing/domain/billingexpr"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/tidwall/gjson"
)

func normalizeChannelTestEndpoint(channel *gatewayschema.Channel, modelName, endpointType string) string {
	normalized := strings.TrimSpace(endpointType)
	if normalized != "" {
		return normalized
	}
	if strings.HasSuffix(modelName, gatewaystore.CompactModelSuffix) {
		return string(constant.EndpointTypeOpenAIResponseCompact)
	}
	if channel != nil && channel.Type == constant.ChannelTypeCodex {
		return string(constant.EndpointTypeOpenAIResponse)
	}
	if gatewaycontract.IsImageGenerationModel(modelName) {
		return string(constant.EndpointTypeImageGeneration)
	}
	return normalized
}

func attachTestBillingRequestInput(info *relaycommon.RelayInfo, request dto.Request) error {
	if info == nil {
		return nil
	}

	input, err := relaycommon.BuildBillingExprRequestInputFromRequest(request, info.RequestHeaders)
	if err != nil {
		return err
	}
	info.BillingRequestInput = &input
	return nil
}

func settleTestQuota(info *relaycommon.RelayInfo, priceData types.PriceData, usage *dto.Usage) (int, *billingexpr.TieredResult) {
	if usage != nil && info != nil && info.TieredBillingSnapshot != nil {
		isClaudeUsageSemantic := usage.UsageSemantic == "anthropic" || info.GetFinalRequestRelayFormat() == types.RelayFormatClaude
		usedVars := billingexpr.UsedVars(info.TieredBillingSnapshot.ExprString)
		if ok, quota, result := billingapp.TryTieredSettle(info, billingapp.BuildTieredTokenParams(usage, isClaudeUsageSemantic, usedVars)); ok {
			return quota, result
		}
	}

	quota := 0
	if !priceData.UsePrice {
		quota = usage.PromptTokens + int(math.Round(float64(usage.CompletionTokens)*priceData.CompletionRatio))
		quota = int(math.Round(float64(quota) * priceData.ModelRatio))
		if priceData.ModelRatio != 0 && quota <= 0 {
			quota = 1
		}
		return quota, nil
	}

	return int(priceData.ModelPrice * platformruntime.QuotaPerUnit), nil
}

func buildTestLogOther(c *gin.Context, info *relaycommon.RelayInfo, priceData types.PriceData, usage *dto.Usage, tieredResult *billingexpr.TieredResult) map[string]interface{} {
	other := buildGatewayTextOtherInfo(
		c,
		info,
		priceData.ModelRatio,
		priceData.GroupRatioInfo.GroupRatio,
		priceData.CompletionRatio,
		usage.PromptTokensDetails.CachedTokens,
		priceData.CacheRatio,
		priceData.ModelPrice,
		priceData.GroupRatioInfo.GroupSpecialRatio,
	)
	if tieredResult != nil {
		injectGatewayTieredBillingInfo(other, info, tieredResult)
	}
	return other
}

func coerceTestUsage(usageAny any, isStream bool, estimatePromptTokens int) (*dto.Usage, error) {
	switch usage := usageAny.(type) {
	case *dto.Usage:
		return usage, nil
	case dto.Usage:
		return &usage, nil
	case nil:
		if !isStream {
			return nil, errors.New("usage is nil")
		}
	default:
		if !isStream {
			return nil, fmt.Errorf("invalid usage type: %T", usageAny)
		}
	}

	usage := &dto.Usage{
		PromptTokens: estimatePromptTokens,
	}
	usage.TotalTokens = usage.PromptTokens
	return usage, nil
}

func readTestResponseBody(body io.ReadCloser, isStream bool) ([]byte, error) {
	defer func() { _ = body.Close() }()
	const maxStreamLogBytes = 8 << 10
	if isStream {
		return io.ReadAll(io.LimitReader(body, maxStreamLogBytes))
	}
	return io.ReadAll(body)
}

func detectErrorMessageFromJSONBytes(jsonBytes []byte) string {
	if len(jsonBytes) == 0 {
		return ""
	}
	if jsonBytes[0] != '{' && jsonBytes[0] != '[' {
		return ""
	}

	errVal := gjson.GetBytes(jsonBytes, "error")
	if !errVal.Exists() || errVal.Type == gjson.Null {
		return ""
	}

	message := gjson.GetBytes(jsonBytes, "error.message").String()
	if message == "" {
		message = gjson.GetBytes(jsonBytes, "error.error.message").String()
	}
	if message == "" && errVal.Type == gjson.String {
		message = errVal.String()
	}
	if message == "" {
		message = errVal.Raw
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "upstream returned error payload"
	}
	return message
}

func detectErrorFromTestResponseBody(respBody []byte) error {
	body := bytes.TrimSpace(respBody)
	if len(body) == 0 {
		return nil
	}
	if message := detectErrorMessageFromJSONBytes(body); message != "" {
		return fmt.Errorf("upstream error: %s", message)
	}

	for _, line := range bytes.Split(body, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if message := detectErrorMessageFromJSONBytes(payload); message != "" {
			return fmt.Errorf("upstream error: %s", message)
		}
	}

	return nil
}

func validateStreamTestResponseBody(respBody []byte) error {
	body := bytes.TrimSpace(respBody)
	if len(body) == 0 {
		return errors.New("stream response body is empty")
	}

	for _, line := range bytes.Split(body, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		return nil
	}

	return errors.New("stream response body does not contain a valid stream event")
}

func validateTestResponseBody(respBody []byte, isStream bool) error {
	if bodyErr := detectErrorFromTestResponseBody(respBody); bodyErr != nil {
		return bodyErr
	}
	if isStream {
		return validateStreamTestResponseBody(respBody)
	}
	return nil
}

func shouldUseStreamForAutomaticChannelTest(channel *gatewayschema.Channel) bool {
	return channel != nil && channel.Type == constant.ChannelTypeCodex
}

func buildTestRequest(modelName string, endpointType string, channel *gatewayschema.Channel, isStream bool) dto.Request {
	testResponsesInput := json.RawMessage(`[{"role":"user","content":"hi"}]`)

	if endpointType != "" {
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeEmbeddings:
			return &dto.EmbeddingRequest{
				Model: modelName,
				Input: []any{"hello world"},
			}
		case constant.EndpointTypeImageGeneration:
			return &dto.ImageRequest{
				Model:  modelName,
				Prompt: "a cute cat",
				N:      lo.ToPtr(uint(1)),
				Size:   "1024x1024",
			}
		case constant.EndpointTypeJinaRerank:
			return &dto.RerankRequest{
				Model:     modelName,
				Query:     "What is Deep Learning?",
				Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
				TopN:      lo.ToPtr(2),
			}
		case constant.EndpointTypeOpenAIResponse:
			return &dto.OpenAIResponsesRequest{
				Model:  modelName,
				Input:  json.RawMessage(`[{"role":"user","content":"hi"}]`),
				Stream: lo.ToPtr(isStream),
			}
		case constant.EndpointTypeOpenAIResponseCompact:
			return &dto.OpenAIResponsesCompactionRequest{
				Model: modelName,
				Input: testResponsesInput,
			}
		case constant.EndpointTypeAnthropic, constant.EndpointTypeGemini, constant.EndpointTypeOpenAI:
			maxTokens := uint(16)
			if constant.EndpointType(endpointType) == constant.EndpointTypeGemini {
				maxTokens = 3000
			}
			req := &dto.GeneralOpenAIRequest{
				Model:  modelName,
				Stream: lo.ToPtr(isStream),
				Messages: []dto.Message{
					{
						Role:    "user",
						Content: "hi",
					},
				},
				MaxTokens: lo.ToPtr(maxTokens),
			}
			if isStream {
				req.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
			}
			return req
		}
	}

	if strings.Contains(strings.ToLower(modelName), "rerank") {
		return &dto.RerankRequest{
			Model:     modelName,
			Query:     "What is Deep Learning?",
			Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
			TopN:      lo.ToPtr(2),
		}
	}
	if strings.Contains(strings.ToLower(modelName), "embedding") ||
		strings.HasPrefix(modelName, "m3e") ||
		strings.Contains(modelName, "bge-") {
		return &dto.EmbeddingRequest{
			Model: modelName,
			Input: []any{"hello world"},
		}
	}
	if strings.HasSuffix(modelName, gatewaystore.CompactModelSuffix) {
		return &dto.OpenAIResponsesCompactionRequest{
			Model: modelName,
			Input: testResponsesInput,
		}
	}
	if strings.Contains(strings.ToLower(modelName), "codex") {
		return &dto.OpenAIResponsesRequest{
			Model:  modelName,
			Input:  json.RawMessage(`[{"role":"user","content":"hi"}]`),
			Stream: lo.ToPtr(isStream),
		}
	}

	testRequest := &dto.GeneralOpenAIRequest{
		Model:  modelName,
		Stream: lo.ToPtr(isStream),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}
	if isStream {
		testRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}

	if strings.HasPrefix(modelName, "o") {
		testRequest.MaxCompletionTokens = lo.ToPtr(uint(16))
	} else if strings.Contains(modelName, "thinking") {
		if !strings.Contains(modelName, "claude") {
			testRequest.MaxTokens = lo.ToPtr(uint(50))
		}
	} else if strings.Contains(modelName, "gemini") {
		testRequest.MaxTokens = lo.ToPtr(uint(3000))
	} else {
		testRequest.MaxTokens = lo.ToPtr(uint(16))
	}

	_ = channel
	return testRequest
}

func normalizeChannelTestModel(channel *gatewayschema.Channel, testModel string) string {
	testModel = strings.TrimSpace(testModel)
	if testModel != "" {
		return testModel
	}
	if channel.TestModel != nil && *channel.TestModel != "" {
		return strings.TrimSpace(*channel.TestModel)
	}
	models := channel.GetModels()
	if len(models) > 0 {
		testModel = strings.TrimSpace(models[0])
	}
	if testModel == "" {
		testModel = "gpt-4o-mini"
	}
	return testModel
}

func resolveChannelTestRequestPath(channel *gatewayschema.Channel, modelName string, endpointType string) string {
	requestPath := "/v1/chat/completions"

	if endpointType != "" {
		if endpointInfo, ok := gatewaycontract.DefaultEndpointInfo(constant.EndpointType(endpointType)); ok {
			return endpointInfo.Path
		}
		return requestPath
	}

	if strings.Contains(strings.ToLower(modelName), "rerank") {
		requestPath = "/v1/rerank"
	}
	if strings.Contains(strings.ToLower(modelName), "embedding") ||
		strings.HasPrefix(modelName, "m3e") ||
		strings.Contains(modelName, "bge-") ||
		strings.Contains(modelName, "embed") ||
		channel.Type == constant.ChannelTypeMokaAI {
		requestPath = "/v1/embeddings"
	}
	if channel.Type == constant.ChannelTypeVolcEngine && strings.Contains(modelName, "seedream") {
		requestPath = "/v1/images/generations"
	}
	if strings.Contains(strings.ToLower(modelName), "codex") {
		requestPath = "/v1/responses"
	}
	if strings.HasSuffix(modelName, gatewaystore.CompactModelSuffix) {
		requestPath = "/v1/responses/compact"
	}
	return requestPath
}

func resolveChannelTestRelayFormat(endpointType string, requestPath string) types.RelayFormat {
	if endpointType != "" {
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeOpenAI:
			return types.RelayFormatOpenAI
		case constant.EndpointTypeOpenAIResponse:
			return types.RelayFormatOpenAIResponses
		case constant.EndpointTypeOpenAIResponseCompact:
			return types.RelayFormatOpenAIResponsesCompaction
		case constant.EndpointTypeAnthropic:
			return types.RelayFormatClaude
		case constant.EndpointTypeGemini:
			return types.RelayFormatGemini
		case constant.EndpointTypeJinaRerank:
			return types.RelayFormatRerank
		case constant.EndpointTypeImageGeneration:
			return types.RelayFormatOpenAIImage
		case constant.EndpointTypeEmbeddings:
			return types.RelayFormatEmbedding
		default:
			return types.RelayFormatOpenAI
		}
	}

	relayFormat := types.RelayFormatOpenAI
	switch {
	case requestPath == "/v1/embeddings":
		relayFormat = types.RelayFormatEmbedding
	case requestPath == "/v1/images/generations":
		relayFormat = types.RelayFormatOpenAIImage
	case requestPath == "/v1/messages":
		relayFormat = types.RelayFormatClaude
	case strings.Contains(requestPath, "/v1beta/models"):
		relayFormat = types.RelayFormatGemini
	case requestPath == "/v1/rerank" || requestPath == "/rerank":
		relayFormat = types.RelayFormatRerank
	case requestPath == "/v1/responses":
		relayFormat = types.RelayFormatOpenAIResponses
	case strings.HasPrefix(requestPath, "/v1/responses/compact"):
		relayFormat = types.RelayFormatOpenAIResponsesCompaction
	}
	return relayFormat
}

func getChannelForTest(id int) (*gatewayschema.Channel, error) {
	channel, err := gatewaystore.GetCachedChannel(id)
	if err == nil {
		return channel, nil
	}
	return gatewaystore.LoadChannelByID(id, true)
}

func loadGatewayUserGroup(userID int, selectAll bool) (string, error) {
	return identitystore.LoadUserGroup(userID, selectAll)
}

func writeGatewayUserCacheToContext(c *gin.Context, userID int) error {
	return identitystore.WriteUserCacheToContext(c, userID)
}

func parseChannelTestID(value string) (int, error) {
	return strconv.Atoi(value)
}

func buildChannelTestRequestURL(path string) *url.URL {
	return &url.URL{Path: path}
}
