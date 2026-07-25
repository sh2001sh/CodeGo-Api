package app

import (
	"context"
	"fmt"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func PreConsumeRelayBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.APIError {
	if relayInfo != nil && relayInfo.Billing != nil {
		if err := relayInfo.Billing.Reserve(preConsumedQuota); err != nil {
			if apiErr, ok := err.(*types.APIError); ok {
				return apiErr
			}
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		return nil
	}

	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

func RefundRelayBillingSync(c *gin.Context, relayInfo *relaycommon.RelayInfo) error {
	if relayInfo == nil || relayInfo.Billing == nil {
		return nil
	}
	if session, ok := relayInfo.Billing.(*BillingSession); ok {
		if err := session.RefundSync(c); err != nil {
			return err
		}
	} else {
		relayInfo.Billing.Refund(c)
	}
	relayInfo.Billing = nil
	relayInfo.FinalPreConsumedQuota = 0
	relayInfo.BillingSource = ""
	return nil
}

func SettleRelayBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}
		if session, ok := relayInfo.Billing.(*BillingSession); ok {
			if funding, ok := session.funding.(settlementProjectionFunding); ok {
				startRequestSettlementProjection(ctx, relayInfo, funding, actualQuota)
			}
		}

		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}

type settlementProjectionFunding interface {
	AccountID() string
	ReservationID() string
	SettlementID() string
}

func startRequestSettlementProjection(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, funding settlementProjectionFunding, actualQuota int) {
	if relayInfo == nil || funding == nil || funding.SettlementID() == "" {
		return
	}
	params := RequestSettlementWorkflowParams{
		RequestID:       relayInfo.RequestId,
		TraceID:         traceIDFromContext(ctx),
		UserID:          relayInfo.UserId,
		TokenID:         relayInfo.TokenId,
		AccountID:       funding.AccountID(),
		ReservationID:   funding.ReservationID(),
		SettlementID:    funding.SettlementID(),
		UsageEvidenceID: relayInfo.RequestId,
		ReservedAmount:  int64(relayInfo.FinalPreConsumedQuota),
		ActualAmount:    int64(actualQuota),
	}
	gopool.Go(func() {
		if err := StartRequestSettlementWorkflow(context.Background(), params); err != nil {
			platformobservability.SysError("schedule request settlement workflow: " + err.Error())
		}
	})
}

func traceIDFromContext(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.GetString(constant.TraceIdKey)
}
