package execution

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	gatewaytranslation "github.com/sh2001sh/CodeGo-Api/internal/gateway/translation"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformcopy "github.com/sh2001sh/CodeGo-Api/internal/platform/copyx"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
)

func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (apiError *types.APIError) {
	info.InitChannelMeta(c)

	textReq, ok := info.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.GeneralOpenAIRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := platformcopy.DeepCopy(textReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if request.WebSearchOptions != nil {
		c.Set("chat_completion_web_search_context_size", request.WebSearchOptions.SearchContextSize)
	}
	if err := relaycommon.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	includeUsage := true
	if request.StreamOptions != nil {
		includeUsage = request.StreamOptions.IncludeUsage
	}
	if !info.SupportStreamOptions || !lo.FromPtrOr(request.Stream, false) {
		request.StreamOptions = nil
	} else if constant.ForceStreamOption {
		request.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	info.ShouldIncludeUsage = includeUsage

	adaptor := NewSyncAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	passThroughGlobal := gatewaystore.GetGlobalSettings().PassThroughRequestEnabled
	if info.RelayMode == gatewaycontract.RelayModeChatCompletions &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		gatewaytranslation.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.ChannelType, info.OriginModelName) {
		applySystemPromptIfNeeded(c, info, request)
		usage, apiError := chatCompletionsViaResponses(c, info, adaptor, request)
		if apiError != nil {
			return apiError
		}

		containAudioTokens := usage.CompletionTokenDetails.AudioTokens > 0 || usage.PromptTokensDetails.AudioTokens > 0
		containsAudioRatios := gatewaystore.ContainsAudioRatio(info.OriginModelName) || gatewaystore.ContainsAudioCompletionRatio(info.OriginModelName)
		if containAudioTokens && containsAudioRatios {
			billingapp.PostAudioConsumeQuota(c, info, usage, "")
		} else {
			billingapp.PostTextConsumeQuota(c, info, usage, nil)
		}
		return nil
	}

	var requestBody io.Reader
	if passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := platformhttpx.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if platformconfig.DebugEnabled {
			if debugBytes, bodyErr := storage.Bytes(); bodyErr == nil {
				println("requestBody: ", string(debugBytes))
			}
		}
		requestBody = platformhttpx.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		if info.ChannelSetting.SystemPrompt != "" {
			request, ok := convertedRequest.(*dto.GeneralOpenAIRequest)
			if ok {
				containSystemPrompt := false
				for _, message := range request.Messages {
					if message.Role == request.GetSystemRoleName() {
						containSystemPrompt = true
						break
					}
				}
				if !containSystemPrompt {
					request.Messages = append([]dto.Message{{
						Role:    request.GetSystemRoleName(),
						Content: info.ChannelSetting.SystemPrompt,
					}}, request.Messages...)
				} else if info.ChannelSetting.SystemPromptOverride {
					httpctx.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
					for i, message := range request.Messages {
						if message.Role != request.GetSystemRoleName() {
							continue
						}
						if message.IsStringContent() {
							request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + message.StringContent())
						} else {
							contents := message.ParseContent()
							contents = append([]dto.MediaContent{{
								Type: dto.ContentTypeText,
								Text: info.ChannelSetting.SystemPrompt,
							}}, contents...)
							request.Messages[i].Content = contents
						}
						break
					}
				}
			}
		}

		jsonData, err := platformencoding.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
		}

		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return apiErrorFromParamOverride(err)
			}
		}

		logger.LogDebug(c, fmt.Sprintf("text request body: %s", string(jsonData)))
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			apiError := platformhttpx.RelayErrorHandler(c.Request.Context(), httpResp, false)
			platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
			return apiError
		}
	}

	usage, apiError := adaptor.DoResponse(c, httpResp, info)
	if apiError != nil {
		platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
		return apiError
	}

	usageDTO := usage.(*dto.Usage)
	containAudioTokens := usageDTO.CompletionTokenDetails.AudioTokens > 0 || usageDTO.PromptTokensDetails.AudioTokens > 0
	containsAudioRatios := gatewaystore.ContainsAudioRatio(info.OriginModelName) || gatewaystore.ContainsAudioCompletionRatio(info.OriginModelName)
	if containAudioTokens && containsAudioRatios {
		billingapp.PostAudioConsumeQuota(c, info, usageDTO, "")
	} else {
		billingapp.PostTextConsumeQuota(c, info, usageDTO, nil)
	}
	return nil
}
