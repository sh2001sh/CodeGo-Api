package aws

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/claude"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformfilex "github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type ClientMode int

const (
	ClientModeApiKey ClientMode = iota + 1
	ClientModeAKSK
)

type Adaptor struct {
	ClientMode ClientMode
	AwsClient  *bedrockruntime.Client
	AwsModelId string
	AwsReq     any
	IsNova     bool
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, _ *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	for i, message := range request.Messages {
		updated := false
		if !message.IsStringContent() {
			content, err := message.ParseContent()
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse message content")
			}
			for j, mediaMessage := range content {
				if mediaMessage.Source != nil && mediaMessage.Source.Type == "url" {
					source := types.NewURLFileSource(mediaMessage.Source.Url)
					base64Data, mimeType, err := platformfilex.GetBase64Data(c, source, "formatting image for Claude")
					if err != nil {
						return nil, fmt.Errorf("get file base64 from url failed: %s", err.Error())
					}
					mediaMessage.Source.MediaType = mimeType
					mediaMessage.Source.Data = base64Data
					mediaMessage.Source.Url = ""
					mediaMessage.Source.Type = "base64"
					content[j] = mediaMessage
					updated = true
				}
			}
			if updated {
				message.SetContent(content)
			}
		}
		if updated {
			request.Messages[i] = message
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(*relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.ChannelOtherSettings.AwsKeyType == dto.AwsKeyTypeApiKey {
		awsModelID := getAwsModelID(info.UpstreamModelName)
		a.ClientMode = ClientModeApiKey
		awsSecret := strings.Split(info.ApiKey, "|")
		if len(awsSecret) != 2 {
			return "", errors.New("invalid aws api key, should be in format of <api-key>|<region>")
		}
		return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse", awsSecret[1], awsModelID), nil
	}

	a.ClientMode = ClientModeAKSK
	return "", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	claude.CommonClaudeHeadersOperation(c, req, info)
	if a.ClientMode == ClientModeApiKey {
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if isNovaModel(request.Model) {
		a.IsNova = true
		return convertToNovaRequest(request), nil
	}

	claudeReq, err := claude.RequestOpenAI2ClaudeMessage(c, *request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert openai request to claude request")
	}
	info.UpstreamModelName = claudeReq.Model
	return claudeReq, nil
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
	if a.ClientMode == ClientModeApiKey {
		return synchttp.DoAPIRequest(a, c, info, requestBody)
	}
	return doAwsClientRequest(c, info, a, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.APIError) {
	if a.ClientMode == ClientModeApiKey {
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	}

	if a.IsNova {
		err, usage = handleNovaRequest(c, info, a)
		return usage, err
	}
	if info.IsStream {
		err, usage = awsStreamHandler(c, info, a)
		return usage, err
	}
	err, usage = awsHandler(c, info, a)
	return usage, err
}

func (a *Adaptor) GetModelList() (models []string) {
	for modelName := range awsModelIDMap {
		models = append(models, modelName)
	}
	return models
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
