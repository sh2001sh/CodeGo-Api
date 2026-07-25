package app

import (
	"context"
	"fmt"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"

	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformmath "github.com/sh2001sh/CodeGo-Api/internal/platform/mathx"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

func taskBillingModelName(task *workflowschema.Task) string {
	if billingContext := task.PrivateData.BillingContext; billingContext != nil && billingContext.OriginModelName != "" {
		return billingContext.OriginModelName
	}
	return task.Properties.OriginModelName
}

func taskBillingIsSubscription(task *workflowschema.Task) bool {
	return task.PrivateData.BillingSource == "subscription" && task.PrivateData.SubscriptionId > 0
}

func taskBillingResolveTokenKey(ctx context.Context, tokenID int, taskID string) string {
	token, err := billingapp.GetTokenByID(tokenID)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenID, taskID, err.Error()))
		return ""
	}
	return token.Key
}

func taskBillingAdjustFunding(task *workflowschema.Task, delta int) error {
	if task == nil || delta == 0 {
		return nil
	}
	if taskBillingIsSubscription(task) {
		return commerceapp.PostConsumeUserSubscriptionUsageDelta(task.PrivateData.SubscriptionId, taskBillingModelName(task), int64(delta))
	}
	return billingapp.AdjustWalletQuota(task.UserId, delta)
}

func taskBillingAdjustTokenQuota(ctx context.Context, task *workflowschema.Task, delta int) {
	if task == nil || task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := taskBillingResolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}

	var err error
	err = billingapp.AdjustTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

func taskBillingOther(task *workflowschema.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if billingContext := task.PrivateData.BillingContext; billingContext != nil {
		other["model_price"] = billingContext.ModelPrice
		if billingContext.ModelRatio > 0 {
			other["model_ratio"] = billingContext.ModelRatio
		}
		other["group_ratio"] = billingContext.GroupRatio
		for key, value := range billingContext.OtherRatios {
			other[key] = value
		}
	}
	if upstreamModelName := task.Properties.UpstreamModelName; upstreamModelName != "" && upstreamModelName != task.Properties.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = upstreamModelName
	}
	return other
}

func recordTaskRefund(ctx context.Context, task *workflowschema.Task, reason string) {
	if task == nil {
		return
	}
	quota := task.Quota
	if quota == 0 {
		return
	}
	if err := taskBillingAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	taskBillingAdjustTokenQuota(ctx, task, -quota)

	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	auditapp.RecordTaskBillingLog(auditschema.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   auditschema.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskBillingModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecordTaskRefundForWorkflow refunds a task from workflow-owned runtime paths.
func RecordTaskRefundForWorkflow(ctx context.Context, task *workflowschema.Task, reason string) {
	recordTaskRefund(ctx, task, reason)
}

func settleTaskQuotaDelta(ctx context.Context, task *workflowschema.Task, actualQuota int, reason string) {
	if task == nil || actualQuota <= 0 {
		return
	}

	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota
	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）", task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf(
		"任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	if err := taskBillingAdjustFunding(task, quotaDelta); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	taskBillingAdjustTokenQuota(ctx, task, quotaDelta)

	task.Quota = actualQuota
	if err := workflowdomain.UpdateTaskQuota(task); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算回写 quota 失败 task %s: %s", task.TaskID, err.Error()))
	}

	logType := auditschema.LogTypeRefund
	logQuota := -quotaDelta
	if quotaDelta > 0 {
		logType = auditschema.LogTypeConsume
		logQuota = quotaDelta
		billingapp.RecordUsageStats(task.UserId, task.ChannelId, quotaDelta)
	}

	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	auditapp.RecordTaskBillingLog(auditschema.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: taskBillingModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

func settleTaskQuotaByTokens(ctx context.Context, task *workflowschema.Task, totalTokens int) {
	if task == nil || totalTokens <= 0 {
		return
	}

	modelName := taskBillingModelName(task)
	modelRatio, hasRatioSetting, _ := gatewaystore.GetModelRatio(modelName)
	if !hasRatioSetting || modelRatio <= 0 {
		return
	}

	group := task.Group
	if group == "" {
		user, err := getWorkflowUserByID(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return
	}

	groupRatio := gatewaystore.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := gatewaystore.GetGroupGroupRatio(group, group)
	finalGroupRatio := groupRatio
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	}

	otherMultiplier := 1.0
	if billingContext := task.PrivateData.BillingContext; billingContext != nil {
		for _, ratio := range billingContext.OtherRatios {
			if ratio > 0 && ratio != 1.0 {
				otherMultiplier *= ratio
			}
		}
	}

	actualQuota := platformmath.SaturatingMulToInt(float64(totalTokens), modelRatio, finalGroupRatio, otherMultiplier)
	reason := fmt.Sprintf(
		"token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f",
		totalTokens,
		modelRatio,
		finalGroupRatio,
		otherMultiplier,
	)
	settleTaskQuotaDelta(ctx, task, actualQuota, reason)
}

func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *workflowschema.Task, taskResult *relaycommon.TaskInfo) {
	if task == nil || taskResult == nil {
		return
	}
	if billingContext := task.PrivateData.BillingContext; billingContext != nil && billingContext.PerCallBilling {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，跳过差额结算", task.TaskID))
		return
	}
	if adaptor != nil {
		if actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult); actualQuota > 0 {
			settleTaskQuotaDelta(ctx, task, actualQuota, "adaptor计费调整")
			return
		}
	}
	if taskResult.TotalTokens > 0 {
		settleTaskQuotaByTokens(ctx, task, taskResult.TotalTokens)
	}
}
