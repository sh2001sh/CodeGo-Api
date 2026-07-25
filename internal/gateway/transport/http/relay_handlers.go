package http

import (
	"errors"
	"fmt"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sh2001sh/CodeGo-Api/constant"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	requestsettings "github.com/sh2001sh/CodeGo-Api/internal/platform/requestsettings"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// RelayWithFormat handles the main synchronous relay entrypoints for a specific protocol shape.
func RelayWithFormat(relayFormat types.RelayFormat) gin.HandlerFunc {
	return func(c *gin.Context) {
		relayRequest(c, relayFormat)
	}
}

// Playground handles authenticated playground text requests.
func Playground(c *gin.Context) {
	if c.GetBool("use_access_token") {
		respondRelayError(c, types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry()))
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		respondRelayError(c, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry()))
		return
	}

	userID := c.GetInt("id")
	if err := identityapp.WriteUserCacheToContext(c, userID); err != nil {
		respondRelayError(c, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry()))
		return
	}

	tempToken := &identityschema.Token{
		UserId: userID,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	relayRequest(c, types.RelayFormatOpenAI)
}

// PlaygroundImage handles authenticated image workspace relay requests.
func PlaygroundImage(c *gin.Context) {
	if c.GetBool("use_access_token") {
		respondRelayError(c, types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry()))
		return
	}

	meta, err := identityapp.BuildImageWorkspaceMetaFromRequest(c)
	if err != nil {
		respondRelayError(c, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry()))
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, nil, nil)
	if err != nil {
		respondRelayError(c, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry()))
		return
	}

	userID := c.GetInt("id")
	if err := identityapp.WriteUserCacheToContext(c, userID); err != nil {
		respondRelayError(c, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry()))
		return
	}

	tempToken := &identityschema.Token{
		UserId: userID,
		Name:   fmt.Sprintf("image-workspace-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	c.Set(string(constant.ContextKeyImageWorkspaceCaptureResponse), true)
	relayRequest(c, types.RelayFormatOpenAIImage)

	if c.Writer.Status() >= http.StatusBadRequest {
		return
	}
	rawResponse, ok := c.Get(string(constant.ContextKeyImageWorkspaceResponseBody))
	if !ok {
		return
	}
	responseBody, ok := rawResponse.([]byte)
	if !ok || len(responseBody) == 0 {
		return
	}
	_, _ = identityapp.PersistImageWorkspaceResponse(c, meta, responseBody)
}

// RelayNotImplemented returns a standard OpenAI-style "not implemented" response.
func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "codego_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

// RelayNotFound returns a standard OpenAI-style "invalid URL" response.
func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func relayRequest(c *gin.Context, relayFormat types.RelayFormat) {
	requestID := c.GetString(constant.RequestIdKey)

	var (
		apiError  *types.APIError
		relayInfo *relaycommon.RelayInfo
	)

	ws, err := upgradeRelayWebsocket(c, relayFormat)
	if err != nil {
		return
	}
	if ws != nil {
		defer ws.Close()
	}

	defer func() {
		apiError = refundRelayBillingIfNeeded(c, relayInfo, apiError)
		if apiError != nil {
			recordRelayFailure(relayInfo)
		}
		finalizeRelayError(c, relayFormat, ws, apiError, requestID)
	}()

	request, err := getAndValidateRequest(c, relayFormat)
	if err != nil {
		if platformhttpx.IsRequestBodyTooLargeError(err) || errors.Is(err, platformhttpx.ErrRequestBodyTooLarge) {
			apiError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			apiError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		apiError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	needSensitiveCheck := requestsettings.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken

	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := identityapp.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			apiError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := tokenx.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		apiError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}
	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := relaycommon.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		apiError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}
	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("妯″瀷 %s 鍏嶈垂锛岃烦杩囬鎵ｈ垂", relayInfo.OriginModelName))
	}

	retryParam := &gatewayroutingapp.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      platformruntime.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= platformconfig.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			apiError = channelErr
			break
		}
		relayInfo.InitChannelMeta(c)

		currentPriceData, priceErr := relaycommon.ModelPriceHelper(c, relayInfo, tokens, meta)
		if priceErr != nil {
			apiError = types.NewError(priceErr, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
			break
		}

		addUsedChannel(c, channel.Id)
		if bodyErr := restoreRelayRequestBody(c); bodyErr != nil {
			apiError = bodyErr
			break
		}

		if !currentPriceData.FreeModel {
			apiError = billingapp.PreConsumeRelayBilling(c, currentPriceData.QuotaToPreConsume, relayInfo)
			if apiError != nil {
				break
			}
		}

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			apiError = gatewayexecutionapp.ExecuteRealtimeRelay(c, relayInfo)
		case types.RelayFormatClaude:
			apiError = gatewayexecutionapp.ExecuteClaudeRelay(c, relayInfo)
		case types.RelayFormatGemini:
			apiError = geminiRelayHandler(c, relayInfo)
		default:
			apiError = relayHandler(c, relayInfo)
		}

		if apiError == nil {
			gatewayroutingapp.RecordAutoGroupSuccess(c, relayInfo.OriginModelName)
			ttft := relayInfo.FirstResponseTime.Sub(relayInfo.StartTime)
			if !relayInfo.HasSendResponse() {
				ttft = 0
			}
			relaycommon.RecordChannelSuccess(channel.Id, relayInfo.OriginModelName, ttft)
			relayInfo.LastError = nil
			return
		}

		apiError = billingapp.NormalizeViolationFeeError(apiError)
		relayInfo.LastError = apiError
		gatewayexecutionapp.ProcessChannelError(c,
			*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, httpctx.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
			apiError,
		)

		if !shouldRetry(c, apiError, platformconfig.RetryTimes-retryParam.GetRetry()) {
			break
		}
		relaycommon.RecordRouteDecisionRetry(c)
		gatewayroutingapp.RecordAutoGroupFailure(c, relayInfo.OriginModelName)
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("retry channels: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
}

func upgradeRelayWebsocket(c *gin.Context, relayFormat types.RelayFormat) (*websocket.Conn, error) {
	if relayFormat != types.RelayFormatOpenAIRealtime {
		return nil, nil
	}
	ws, err := relayUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		relaycommon.WssError(c, nil, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
		return nil, err
	}
	return ws, nil
}

func respondRelayError(c *gin.Context, apiError *types.APIError) {
	if apiError == nil {
		return
	}
	apiError.SanitizeDownstreamResponse()
	c.JSON(apiError.StatusCode, gin.H{
		"error": apiError.ToOpenAIError(),
	})
}
