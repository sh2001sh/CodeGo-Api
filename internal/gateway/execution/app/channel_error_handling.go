package app

import (
	"fmt"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// ProcessChannelError applies shared disable/logging behavior for channel failures.
func ProcessChannelError(c *gin.Context, channelError types.ChannelError, err *types.APIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, err.Error()))

	modelName := c.GetString("original_model")
	failureClass := classifyUpstreamFailure(err)
	modelScopedFailure := failureClass == upstreamFailureModelUnavailable || failureClass == upstreamFailureAccountExhausted
	if modelScopedFailure && modelName != "" {
		group := selectedChannelGroup(c)
		alternative, lookupErr := gatewaystore.HasAlternativeEnabledAbility(channelError.ChannelId, group, modelName)
		if lookupErr != nil {
			platformobservability.SysError(fmt.Sprintf("检查通道「%s」（#%d）的模型 %s 备用路由失败：%v", channelError.ChannelName, channelError.ChannelId, modelName, lookupErr))
		} else if alternative {
			cooling := coolModelScopedUpstreamFailure(channelError.ChannelId, modelName, c.GetString(constant.RequestIdKey), err)
			c.Set("model_unavailable_with_alternative", true)
			if cooling {
				platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）的模型 %s 上游不可用，已临时冷却该模型路由并切换备用渠道", channelError.ChannelName, channelError.ChannelId, modelName))
			} else {
				platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）的模型 %s 第 %d 次连续失败，保留试错空间", channelError.ChannelName, channelError.ChannelId, modelName, channelHealthFailureCount(channelError.ChannelId, modelName)))
			}
		} else {
			platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）的模型 %s 是唯一可用路由，保留渠道与模型路由", channelError.ChannelName, channelError.ChannelId, modelName))
		}
	} else if ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}
	if isRetryableChannelFailure(err) && !modelScopedFailure {
		gatewayruntime.RecordChannelRetryableFailureWithCooldown(channelError.ChannelId, c.GetString("original_model"), gatewayruntime.RetryableFailureCooldown(err.StatusCode))
		gatewayruntime.InvalidateChannelAffinityForCurrentRequest(c)
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		userID := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenID := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelID := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["upstream_failure_class"] = failureClass
		other["channel_id"] = channelID
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")

		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		if httpctx.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = httpctx.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		gatewayruntime.AppendChannelAffinityAdminInfo(c, adminInfo)
		if decision, ok := gatewayruntime.GetRouteDecision(c); ok {
			adminInfo["route_decision"] = decision
		}
		other["admin_info"] = adminInfo

		startTime := httpctx.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		auditapp.RecordErrorLog(
			c,
			userID,
			channelID,
			modelName,
			tokenName,
			err.MaskSensitiveErrorWithStatusCode(),
			tokenID,
			useTimeSeconds,
			httpctx.GetContextKeyBool(c, constant.ContextKeyIsStream),
			userGroup,
			other,
		)
	}
}

func coolModelScopedUpstreamFailure(channelID int, modelName string, requestID string, err *types.APIError) bool {
	if IsModelUnavailableError(err) {
		return gatewayruntime.RecordChannelModelUnavailable(channelID, modelName, requestID)
	}
	return gatewayruntime.CoolChannelModelForUpstreamFailure(channelID, modelName)
}

func channelHealthFailureCount(channelID int, modelName string) int {
	state, found := gatewayruntime.GetChannelHealth(channelID, modelName)
	if !found {
		return 0
	}
	return state.ConsecutiveRetryableFailures
}

func selectedChannelGroup(c *gin.Context) string {
	group := httpctx.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group != "" {
		return group
	}
	return httpctx.GetContextKeyString(c, constant.ContextKeyUsingGroup)
}
