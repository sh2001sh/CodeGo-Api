package vertex

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/claude"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/gemini"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/openai"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/reasoning"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
)

const (
	RequestModeClaude     = 1
	RequestModeGemini     = 2
	RequestModeOpenSource = 3
)

var claudeModelMap = map[string]string{
	"claude-3-sonnet-20240229":   "claude-3-sonnet@20240229",
	"claude-3-opus-20240229":     "claude-3-opus@20240229",
	"claude-3-haiku-20240307":    "claude-3-haiku@20240307",
	"claude-3-5-sonnet-20240620": "claude-3-5-sonnet@20240620",
	"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-v2@20241022",
	"claude-3-7-sonnet-20250219": "claude-3-7-sonnet@20250219",
	"claude-sonnet-4-20250514":   "claude-sonnet-4@20250514",
	"claude-opus-4-20250514":     "claude-opus-4@20250514",
	"claude-opus-4-1-20250805":   "claude-opus-4-1@20250805",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5@20250929",
	"claude-haiku-4-5-20251001":  "claude-haiku-4-5@20251001",
	"claude-opus-4-5-20251101":   "claude-opus-4-5@20251101",
	"claude-opus-4-6":            "claude-opus-4-6",
	"claude-opus-4-7":            "claude-opus-4-7",
	"claude-opus-4-8":            "claude-opus-4-8",
}

const anthropicVersion = "vertex-2023-10-16"

type Adaptor struct {
	RequestMode        int
	AccountCredentials Credentials
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if gatewaystore.GetGeminiSettings().RemoveFunctionResponseIdEnabled {
		removeFunctionResponseID(request)
	}
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.ConvertGeminiRequest(c, info, request)
}

func removeFunctionResponseID(request *dto.GeminiChatRequest) {
	if request == nil {
		return
	}

	for i := range request.Contents {
		for j := range request.Contents[i].Parts {
			part := &request.Contents[i].Parts[j]
			if part.FunctionResponse != nil && len(part.FunctionResponse.ID) > 0 {
				part.FunctionResponse.ID = nil
			}
		}
	}

	for i := range request.Requests {
		removeFunctionResponseID(&request.Requests[i])
	}
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if v, ok := claudeModelMap[info.UpstreamModelName]; ok {
		c.Set("request_model", v)
	} else {
		c.Set("request_model", request.Model)
	}
	return copyRequest(request, anthropicVersion), nil
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.ConvertImageRequest(c, info, request)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	if strings.HasPrefix(info.UpstreamModelName, "claude") {
		a.RequestMode = RequestModeClaude
	} else if strings.Contains(info.UpstreamModelName, "llama") || strings.Contains(info.UpstreamModelName, "-maas") {
		a.RequestMode = RequestModeOpenSource
	} else {
		a.RequestMode = RequestModeGemini
	}
}

func (a *Adaptor) getRequestURL(info *relaycommon.RelayInfo, modelName, suffix string) (string, error) {
	region := GetModelRegion(info.ApiVersion, info.OriginModelName)
	if info.ChannelOtherSettings.VertexKeyType != dto.VertexKeyTypeAPIKey {
		adc := &Credentials{}
		if err := platformencoding.Unmarshal([]byte(info.ApiKey), adc); err != nil {
			return "", fmt.Errorf("failed to decode credentials file: %w", err)
		}
		a.AccountCredentials = *adc

		switch a.RequestMode {
		case RequestModeGemini:
			return BuildGoogleModelURL(info.ChannelBaseUrl, DefaultAPIVersion, adc.ProjectID, region, modelName, suffix), nil
		case RequestModeClaude:
			return BuildAnthropicModelURL(info.ChannelBaseUrl, DefaultAPIVersion, adc.ProjectID, region, modelName, suffix), nil
		case RequestModeOpenSource:
			return BuildOpenSourceChatCompletionsURL(info.ChannelBaseUrl, adc.ProjectID, region), nil
		}
	} else {
		keyPrefix := "?"
		if strings.HasSuffix(suffix, "?alt=sse") {
			keyPrefix = "&"
		}
		switch a.RequestMode {
		case RequestModeGemini:
			return fmt.Sprintf(
				"%s%skey=%s",
				BuildGoogleModelURL(info.ChannelBaseUrl, DefaultAPIVersion, "", region, modelName, suffix),
				keyPrefix,
				info.ApiKey,
			), nil
		case RequestModeClaude:
			return fmt.Sprintf(
				"%s%skey=%s",
				BuildAnthropicModelURL(info.ChannelBaseUrl, DefaultAPIVersion, "", region, modelName, suffix),
				keyPrefix,
				info.ApiKey,
			), nil
		}
	}
	return "", errors.New("unsupported request mode")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.RequestMode == RequestModeGemini {
		if gatewaystore.GetGeminiSettings().ThinkingAdapterEnabled &&
			!gatewaystore.ShouldPreserveThinkingSuffix(info.OriginModelName) {
			if strings.Contains(info.UpstreamModelName, "-thinking-") {
				info.UpstreamModelName = strings.Split(info.UpstreamModelName, "-thinking-")[0]
			} else if strings.HasSuffix(info.UpstreamModelName, "-thinking") {
				info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
			} else if strings.HasSuffix(info.UpstreamModelName, "-nothinking") {
				info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-nothinking")
			} else if baseModel, level, ok := reasoning.TrimEffortSuffix(info.UpstreamModelName); ok && level != "" {
				info.UpstreamModelName = baseModel
			}
		}

		suffix := "generateContent"
		if info.IsStream {
			suffix = "streamGenerateContent?alt=sse"
		}
		if strings.HasPrefix(info.UpstreamModelName, "imagen") {
			suffix = "predict"
		}
		return a.getRequestURL(info, info.UpstreamModelName, suffix)
	}
	if a.RequestMode == RequestModeClaude {
		suffix := "rawPredict"
		if info.IsStream {
			suffix = "streamRawPredict?alt=sse"
		}
		modelName := info.UpstreamModelName
		if v, ok := claudeModelMap[info.UpstreamModelName]; ok {
			modelName = v
		}
		return a.getRequestURL(info, modelName, suffix)
	}
	if a.RequestMode == RequestModeOpenSource {
		return a.getRequestURL(info, "", "")
	}
	return "", errors.New("unsupported request mode")
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	synchttp.SetupAPIRequestHeader(info, c, req)
	if info.ChannelOtherSettings.VertexKeyType != dto.VertexKeyTypeAPIKey {
		accessToken, err := getAccessToken(a, info)
		if err != nil {
			return err
		}
		req.Set("Authorization", "Bearer "+accessToken)
	}
	if a.AccountCredentials.ProjectID != "" {
		req.Set("x-goog-user-project", a.AccountCredentials.ProjectID)
	}
	if strings.Contains(info.UpstreamModelName, "claude") {
		applyClaudeHeadersOperation(c, req, info)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if a.RequestMode == RequestModeGemini && strings.HasPrefix(info.UpstreamModelName, "imagen") {
		prompt := ""
		for _, m := range request.Messages {
			if m.Role == "user" {
				prompt = m.StringContent()
				if prompt != "" {
					break
				}
			}
		}
		if prompt == "" {
			if p, ok := request.Prompt.(string); ok {
				prompt = p
			}
		}
		if prompt == "" {
			return nil, errors.New("prompt is required for image generation")
		}

		imgReq := dto.ImageRequest{
			Model:  request.Model,
			Prompt: prompt,
			N:      lo.ToPtr(uint(1)),
			Size:   "1024x1024",
		}
		if request.N != nil && *request.N > 0 {
			imgReq.N = lo.ToPtr(uint(*request.N))
		}
		if request.Size != "" {
			imgReq.Size = request.Size
		}
		if len(request.ExtraBody) > 0 {
			var extra map[string]any
			if err := json.Unmarshal(request.ExtraBody, &extra); err == nil {
				if n, ok := extra["n"].(float64); ok && n > 0 {
					imgReq.N = lo.ToPtr(uint(n))
				}
				if size, ok := extra["size"].(string); ok {
					imgReq.Size = size
				}
				if ar, ok := extra["aspectRatio"].(string); ok && ar != "" {
					imgReq.Size = ar
				}
				if params, ok := extra["parameters"].(map[string]any); ok {
					if ar, ok := params["aspectRatio"].(string); ok && ar != "" {
						imgReq.Size = ar
					}
				}
			}
		}
		c.Set("request_model", request.Model)
		return a.ConvertImageRequest(c, info, imgReq)
	}
	if a.RequestMode == RequestModeClaude {
		claudeAdaptor := claude.Adaptor{}
		convertedReq, err := claudeAdaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return nil, err
		}
		claudeReq := convertedReq.(*dto.ClaudeRequest)
		c.Set("request_model", claudeReq.Model)
		info.UpstreamModelName = claudeReq.Model
		return copyRequest(claudeReq, anthropicVersion), nil
	}
	if a.RequestMode == RequestModeGemini {
		geminiAdaptor := gemini.Adaptor{}
		geminiRequest, err := geminiAdaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return nil, err
		}
		c.Set("request_model", request.Model)
		return geminiRequest, nil
	}
	if a.RequestMode == RequestModeOpenSource {
		return request, nil
	}
	return nil, errors.New("unsupported request mode")
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return synchttp.DoAPIRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.APIError) {
	claudeAdaptor := claude.Adaptor{}
	geminiAdaptor := gemini.Adaptor{}
	openaiAdaptor := openai.Adaptor{}
	if info.IsStream {
		switch a.RequestMode {
		case RequestModeClaude:
			return claudeAdaptor.DoResponse(c, resp, info)
		case RequestModeGemini:
			return geminiAdaptor.DoResponse(c, resp, info)
		case RequestModeOpenSource:
			return openaiAdaptor.DoResponse(c, resp, info)
		}
	} else {
		switch a.RequestMode {
		case RequestModeClaude:
			return claudeAdaptor.DoResponse(c, resp, info)
		case RequestModeGemini:
			return geminiAdaptor.DoResponse(c, resp, info)
		case RequestModeOpenSource:
			return openaiAdaptor.DoResponse(c, resp, info)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	claudeAdaptor := claude.Adaptor{}
	geminiAdaptor := gemini.Adaptor{}
	modelList := append([]string{}, ModelList...)
	modelList = append(modelList, claudeAdaptor.GetModelList()...)
	modelList = append(modelList, geminiAdaptor.GetModelList()...)
	return modelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func applyClaudeHeadersOperation(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	anthropicBeta := c.Request.Header.Get("anthropic-beta")
	if anthropicBeta != "" {
		req.Set("anthropic-beta", anthropicBeta)
	}
	gatewaystore.GetClaudeSettings().WriteHeaders(info.OriginModelName, req)
}
