package baidu_v2

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/openai"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type Adaptor struct{}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch info.RelayMode {
	case gatewaycontract.RelayModeChatCompletions:
		return fmt.Sprintf("%s/v2/chat/completions", info.ChannelBaseUrl), nil
	case gatewaycontract.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v2/embeddings", info.ChannelBaseUrl), nil
	case gatewaycontract.RelayModeImagesGenerations:
		return fmt.Sprintf("%s/v2/images/generations", info.ChannelBaseUrl), nil
	case gatewaycontract.RelayModeImagesEdits:
		return fmt.Sprintf("%s/v2/images/edits", info.ChannelBaseUrl), nil
	case gatewaycontract.RelayModeRerank:
		return fmt.Sprintf("%s/v2/rerank", info.ChannelBaseUrl), nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	synchttp.SetupAPIRequestHeader(info, c, req)
	keyParts := strings.Split(info.ApiKey, "|")
	if len(keyParts) == 0 || keyParts[0] == "" {
		return errors.New("invalid API key: authorization token is required")
	}
	if len(keyParts) > 1 && keyParts[1] != "" {
		req.Set("appid", keyParts[1])
	}
	req.Set("Authorization", "Bearer "+keyParts[0])
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if strings.HasSuffix(info.UpstreamModelName, "-search") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-search")
		request.Model = info.UpstreamModelName
		if len(request.WebSearch) == 0 {
			toMap := request.ToMap()
			toMap["web_search"] = map[string]any{
				"enable":          true,
				"enable_citation": true,
				"enable_trace":    true,
				"enable_status":   false,
			}
			return toMap, nil
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
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
	adaptor := openai.Adaptor{}
	usage, err = adaptor.DoResponse(c, resp, info)
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
