package execution

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewayproviders "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaytranslation "github.com/sh2001sh/CodeGo-Api/internal/gateway/translation"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
)

func applySystemPromptIfNeeded(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if info == nil || request == nil || info.ChannelSetting.SystemPrompt == "" {
		return
	}

	systemRole := request.GetSystemRoleName()
	for _, message := range request.Messages {
		if message.Role == systemRole {
			if !info.ChannelSetting.SystemPromptOverride {
				return
			}

			httpctx.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
			for i, candidate := range request.Messages {
				if candidate.Role != systemRole {
					continue
				}
				if candidate.IsStringContent() {
					request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + candidate.StringContent())
					return
				}
				contents := candidate.ParseContent()
				contents = append([]dto.MediaContent{{
					Type: dto.ContentTypeText,
					Text: info.ChannelSetting.SystemPrompt,
				}}, contents...)
				request.Messages[i].Content = contents
				return
			}
			return
		}
	}

	request.Messages = append([]dto.Message{{
		Role:    systemRole,
		Content: info.ChannelSetting.SystemPrompt,
	}}, request.Messages...)
}

func chatCompletionsViaResponses(c *gin.Context, info *relaycommon.RelayInfo, adaptor gatewayproviders.SyncAdaptor, request *dto.GeneralOpenAIRequest) (*dto.Usage, *types.APIError) {
	chatJSON, err := platformencoding.Marshal(request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
		if err != nil {
			return nil, apiErrorFromParamOverride(err)
		}
	}

	var overriddenChatReq dto.GeneralOpenAIRequest
	if err := platformencoding.Unmarshal(chatJSON, &overriddenChatReq); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}

	responsesReq, err := gatewaytranslation.ChatCompletionsRequestToResponsesRequest(&overriddenChatReq)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAIResponses)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = gatewaycontract.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *responsesReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := platformencoding.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	httpResp := resp.(*http.Response)
	info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
	if httpResp.StatusCode != http.StatusOK {
		apiError := platformhttpx.RelayErrorHandler(c.Request.Context(), httpResp, false)
		platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
		return nil, apiError
	}

	if info.IsStream {
		usage, apiError := gatewayproviders.OaiResponsesToChatStreamHandler(c, info, httpResp)
		if apiError != nil {
			platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
			return nil, apiError
		}
		return usage, nil
	}

	usage, apiError := gatewayproviders.OaiResponsesToChatHandler(c, info, httpResp)
	if apiError != nil {
		platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
		return nil, apiError
	}
	return usage, nil
}
