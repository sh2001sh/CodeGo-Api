package http

import (
	"errors"
	"fmt"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.APIError {
	return gatewayexecutionapp.ExecuteRelay(c, info)
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.APIError {
	if strings.Contains(c.Request.URL.Path, "embed") {
		return gatewayexecutionapp.ExecuteGeminiEmbeddingRelay(c, info)
	}
	return gatewayexecutionapp.ExecuteGeminiRelay(c, info)
}

var relayUpgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func addUsedChannel(c *gin.Context, channelID int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelID))
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		return r.GetTokenCountMeta()
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *gatewayroutingapp.RetryParam) (*gatewayschema.Channel, *types.APIError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &gatewayschema.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}

	channel, selectGroup, err := gatewayroutingapp.CacheGetRandomSatisfiedChannel(retryParam)
	info.PriceData.GroupRatioInfo = relaycommon.HandleGroupRatio(c, info)
	if err != nil {
		return nil, types.NewError(
			fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）：%s", selectGroup, info.OriginModelName, err.Error()),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if channel == nil {
		return nil, types.NewError(
			fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}

	apiError := gatewayexecutionapp.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if apiError != nil {
		return nil, apiError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *types.APIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if gatewayruntime.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if httpctx.GetContextKeyBool(c, constant.ContextKeyResponseBodyDelivered) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if gatewayexecutionapp.IsModelScopedUpstreamFailure(openaiErr) {
		return c.GetBool("model_unavailable_with_alternative")
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if gatewaystore.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return gatewaystore.ShouldRetryByStatusCode(code)
}

func finalizeRelayError(c *gin.Context, relayFormat types.RelayFormat, ws *websocket.Conn, apiErr *types.APIError, requestID string) {
	if apiErr == nil {
		return
	}
	logger.LogError(c, fmt.Sprintf("relay error: %s", apiErr.Error()))
	if httpctx.GetContextKeyBool(c, constant.ContextKeyResponseBodyDelivered) {
		return
	}
	apiErr.SanitizeDownstreamResponse()
	rawMessageWithRequestID := platformtext.MessageWithRequestID(apiErr.Error(), requestID)
	if types.IsRemoteProviderError(apiErr) {
		rawMessageWithRequestID = platformtext.SanitizeUpstreamProviderErrorMessage(rawMessageWithRequestID)
	}
	apiErr.SetMessage(rawMessageWithRequestID)
	switch relayFormat {
	case types.RelayFormatOpenAIRealtime:
		relaycommon.WssError(c, ws, apiErr.ToOpenAIError())
	case types.RelayFormatClaude:
		c.JSON(apiErr.StatusCode, gin.H{
			"type":  "error",
			"error": apiErr.ToClaudeError(),
		})
	default:
		c.JSON(apiErr.StatusCode, gin.H{
			"error": apiErr.ToOpenAIError(),
		})
	}
}

func refundRelayBillingIfNeeded(c *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.APIError) *types.APIError {
	if apiErr == nil {
		return nil
	}
	apiErr = billingapp.NormalizeViolationFeeError(apiErr)
	if relayInfo.Billing != nil {
		relayInfo.Billing.Refund(c)
	}
	billingapp.ChargeViolationFeeIfNeeded(c, relayInfo, apiErr)
	return apiErr
}

func recordRelayFailure(relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil {
		return
	}
	gopool.Go(func() {
		auditprojection.RecordRelaySample(relayInfo, false, 0)
	})
}

func restoreRelayRequestBody(c *gin.Context) *types.APIError {
	bodyStorage, bodyErr := platformhttpx.GetBodyStorage(c)
	if bodyErr != nil {
		if platformhttpx.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, platformhttpx.ErrRequestBodyTooLarge) {
			return types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		}
		return types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	c.Request.Body = io.NopCloser(bodyStorage)
	return nil
}
