package app

import billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"

func init() {
	billingapp.RegisterSubscriptionFundingHooks(billingapp.SubscriptionFundingHooks{
		PreConsume: func(requestID string, userID int, modelName string, amount int64) (*billingapp.SubscriptionFundingPreConsumeResult, error) {
			result, err := PreConsumeUserSubscription(requestID, userID, modelName, amount)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			return &billingapp.SubscriptionFundingPreConsumeResult{
				UserSubscriptionID: result.UserSubscriptionId,
				PreConsumed:        result.PreConsumed,
				AmountTotal:        result.AmountTotal,
				AmountUsedAfter:    result.AmountUsedAfter,
			}, nil
		},
		ReserveAdditional: func(requestID string, subscriptionID int, modelName string, amount int64) error {
			return ReserveAdditionalSubscriptionQuota(requestID, subscriptionID, modelName, amount)
		},
		GetPlanInfo: func(userSubscriptionID int) (*billingapp.SubscriptionFundingPlanInfo, error) {
			planInfo, err := GetSubscriptionPlanInfoByUserSubscriptionID(userSubscriptionID)
			if err != nil {
				return nil, err
			}
			if planInfo == nil {
				return nil, nil
			}
			return &billingapp.SubscriptionFundingPlanInfo{
				PlanID:    planInfo.PlanId,
				PlanTitle: planInfo.PlanTitle,
			}, nil
		},
		PostConsumeDelta: func(subscriptionID int, modelName string, delta int64) error {
			return PostConsumeUserSubscriptionUsageDelta(subscriptionID, modelName, delta)
		},
		RefundPreConsume: func(requestID string) error {
			return RefundSubscriptionPreConsume(requestID)
		},
		SettleReservation: func(requestID string, subscriptionID int, modelName string, actualAmount int64) error {
			return SettleSubscriptionReservation(requestID, subscriptionID, modelName, actualAmount)
		},
	})
}
