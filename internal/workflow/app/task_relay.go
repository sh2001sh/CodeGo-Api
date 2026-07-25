package app

import (
	"errors"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/taskx"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// FetchRelayTask handles public async task fetch requests for relay/video routes.
func FetchRelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// SubmitRelayTask handles async task submission for relay/video routes.
func SubmitRelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &gatewayroutingapp.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      platformruntime.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= platformconfig.RetryTimes; retryParam.IncreaseRetry() {
		var channel *gatewayschema.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*gatewayschema.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := gatewayexecutionapp.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = taskx.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.APIError
			channel, channelErr = getTaskChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = taskx.TaskErrorWrapper(channelErr.Err, "get_channel_failed", http.StatusServiceUnavailable)
				break
			}
		}

		addUsedTaskChannel(c, channel.Id)
		bodyStorage, bodyErr := platformhttpx.GetBodyStorage(c)
		if bodyErr != nil {
			if platformhttpx.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, platformhttpx.ErrRequestBodyTooLarge) {
				taskErr = taskx.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = taskx.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			gatewayroutingapp.RecordAutoGroupSuccess(c, relayInfo.OriginModelName)
			relaycommon.RecordChannelSuccess(channel.Id, relayInfo.OriginModelName, 0)
			break
		}

		if !taskErr.LocalError {
			gatewayroutingapp.RecordAutoGroupFailure(c, relayInfo.OriginModelName)
			gatewayexecutionapp.ProcessChannelError(
				c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, httpctx.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode),
			)
		}

		if !shouldRetryTaskRelay(c, taskErr, platformconfig.RetryTimes-retryParam.GetRetry()) {
			break
		}
		relaycommon.RecordRouteDecisionRetry(c)
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	if taskErr == nil {
		if settleErr := relayInfo.Billing.Settle(result.Quota); settleErr != nil {
			platformobservability.SysError("settle task billing error: " + settleErr.Error())
		}
		billingapp.LogTaskConsumption(c, relayInfo)

		task := newWorkflowTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		task.PrivateData.RequestID = relayInfo.RequestId
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &workflowschema.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  platformtext.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := workflowdomain.InsertTask(task); insertErr != nil {
			platformobservability.SysError("insert task error: " + insertErr.Error())
		} else if startErr := StartAsyncTaskWorkflow(c.Request.Context(), task); startErr != nil {
			platformobservability.SysError("start async task workflow error: " + startErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "status_code=429"
	}
	if taskErr.Code == string(types.ErrorCodeGetChannelFailed) {
		taskErr.StatusCode = http.StatusServiceUnavailable
		taskErr.Message = types.ModelUnavailableMessage
	}
	if !taskErr.LocalError {
		taskErr.Message = platformtext.SanitizeUpstreamQuotaErrorMessage(taskErr.Message)
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests || taskErr.StatusCode == http.StatusTemporaryRedirect {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		if gatewaystore.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest || taskErr.StatusCode == http.StatusRequestTimeout {
		return false
	}
	if taskErr.LocalError || taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}

func addUsedTaskChannel(c *gin.Context, channelID int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelID))
	c.Set("use_channel", useChannel)
}

func getTaskChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *gatewayroutingapp.RetryParam) (*gatewayschema.Channel, *types.APIError) {
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
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）：%s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	apiError := gatewayexecutionapp.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if apiError != nil {
		return nil, apiError
	}
	return channel, nil
}
