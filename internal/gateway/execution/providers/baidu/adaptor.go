package baidu

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type Adaptor struct{}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	panic("implement me")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	suffix := "chat/"
	if strings.HasPrefix(info.UpstreamModelName, "Embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "bge-large") ||
		strings.HasPrefix(info.UpstreamModelName, "tao-8k") {
		suffix = "embeddings/"
	}

	switch info.UpstreamModelName {
	case "ERNIE-4.0", "ERNIE-Bot-4", "ERNIE-4.0-8K":
		suffix += "completions_pro"
	case "ERNIE-Bot", "ERNIE-3.5-8K":
		suffix += "completions"
	case "ERNIE-Bot-turbo", "ERNIE-Lite-8K-0922":
		suffix += "eb-instant"
	case "ERNIE-Speed", "ERNIE-Speed-8K":
		suffix += "ernie_speed"
	case "ERNIE-3.5-8K-0205":
		suffix += "ernie-3.5-8k-0205"
	case "ERNIE-3.5-8K-1222":
		suffix += "ernie-3.5-8k-1222"
	case "ERNIE-Bot-8K":
		suffix += "ernie_bot_8k"
	case "ERNIE-3.5-4K-0205":
		suffix += "ernie-3.5-4k-0205"
	case "ERNIE-Speed-128K":
		suffix += "ernie-speed-128k"
	case "ERNIE-Lite-8K-0308":
		suffix += "ernie-lite-8k"
	case "ERNIE-Tiny-8K":
		suffix += "ernie-tiny-8k"
	case "BLOOMZ-7B":
		suffix += "bloomz_7b1"
	case "Embedding-V1":
		suffix += "embedding-v1"
	case "bge-large-zh":
		suffix += "bge_large_zh"
	case "bge-large-en":
		suffix += "bge_large_en"
	case "tao-8k":
		suffix += "tao_8k"
	default:
		suffix += strings.ToLower(info.UpstreamModelName)
	}

	fullRequestURL := fmt.Sprintf("%s/rpc/2.0/ai_custom/v1/wenxinworkshop/%s", info.ChannelBaseUrl, suffix)
	accessToken, err := getBaiduAccessToken(info.ApiKey)
	if err != nil {
		return "", err
	}
	return fullRequestURL + "?access_token=" + accessToken, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	synchttp.SetupAPIRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return requestOpenAI2Baidu(*request), nil
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, _ dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return embeddingRequestOpenAI2Baidu(request), nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return synchttp.DoAPIRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.APIError) {
	if info.IsStream {
		err, usage = baiduStreamHandler(c, info, resp)
	} else {
		switch info.RelayMode {
		case gatewaycontract.RelayModeEmbeddings:
			err, usage = baiduEmbeddingHandler(c, info, resp)
		default:
			err, usage = baiduHandler(c, info, resp)
		}
	}
	return usage, err
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
