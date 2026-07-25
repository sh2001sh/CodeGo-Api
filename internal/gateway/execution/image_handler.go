package execution

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformcopy "github.com/sh2001sh/CodeGo-Api/internal/platform/copyx"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (apiError *types.APIError) {
	info.InitChannelMeta(c)

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := platformcopy.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if err := relaycommon.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := NewSyncAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader
	if gatewaystore.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := platformhttpx.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		requestBody = platformhttpx.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch typed := convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = typed
		default:
			jsonData, err := platformencoding.Marshal(convertedRequest)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
				if err != nil {
					return apiErrorFromParamOverride(err)
				}
			}

			logger.LogDebug(c, "image request body: %s", jsonData)
			body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			defer closer.Close()
			info.UpstreamRequestBodySize = size
			requestBody = body
		}
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
				httpResp.StatusCode = http.StatusOK
			} else {
				apiError = platformhttpx.RelayErrorHandler(c.Request.Context(), httpResp, false)
				platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
				return apiError
			}
		}
	}

	usage, apiError := adaptor.DoResponse(c, httpResp, info)
	if apiError != nil {
		platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
		return apiError
	}

	imageN := uint(1)
	if request.N != nil {
		imageN = *request.N
	}
	if info.PriceData.UsePrice {
		if _, hasN := info.PriceData.OtherRatios["n"]; !hasN {
			info.PriceData.AddOtherRatio("n", float64(imageN))
		}
	}

	usageDTO := usage.(*dto.Usage)
	if usageDTO.TotalTokens == 0 {
		usageDTO.TotalTokens = 1
	}
	if usageDTO.PromptTokens == 0 {
		usageDTO.PromptTokens = 1
	}

	quality := "standard"
	if request.Quality == "hd" {
		quality = "hd"
	}

	var logContent []string
	if len(request.Size) > 0 {
		logContent = append(logContent, fmt.Sprintf("澶у皬 %s", request.Size))
	}
	if len(quality) > 0 {
		logContent = append(logContent, fmt.Sprintf("鍝佽川 %s", quality))
	}
	if imageN > 0 {
		logContent = append(logContent, fmt.Sprintf("鐢熸垚鏁伴噺 %d", imageN))
	}

	billingapp.PostTextConsumeQuota(c, info, usageDTO, logContent)
	return nil
}
