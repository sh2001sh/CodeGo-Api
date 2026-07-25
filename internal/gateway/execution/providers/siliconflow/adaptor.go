package siliconflow

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/openai"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
)

type Adaptor struct{}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	adaptor := openai.Adaptor{}
	return adaptor.ConvertAudioRequest(c, info, request)
}

func (a *Adaptor) ConvertImageRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	sfRequest := &SFImageRequest{}
	extra, err := platformencoding.Marshal(request.Extra)
	if err == nil {
		err = platformencoding.Unmarshal(extra, sfRequest)
		if err != nil {
			sfRequest = &SFImageRequest{}
		}
	}

	sfRequest.Model = request.Model
	sfRequest.Prompt = request.Prompt
	if sfRequest.ImageSize == "" {
		sfRequest.ImageSize = request.Size
	}
	if sfRequest.BatchSize == 0 && request.N != nil {
		sfRequest.BatchSize = lo.FromPtr(request.N)
	}

	return sfRequest, nil
}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == gatewaycontract.RelayModeRerank {
		return fmt.Sprintf("%s/v1/rerank", info.ChannelBaseUrl), nil
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	adaptor := openai.Adaptor{}
	return adaptor.SetupRequestHeader(c, req, info)
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if (request.Prefix != nil || request.Suffix != nil) && len(request.Messages) == 0 {
		request.Messages = []dto.Message{
			{
				Role:    "user",
				Content: "",
			},
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	adaptor := openai.Adaptor{}
	return adaptor.DoRequest(c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.APIError) {
	switch info.RelayMode {
	case gatewaycontract.RelayModeRerank:
		return siliconflowRerankHandler(c, info, resp)
	default:
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
