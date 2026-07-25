package app

import (
	"math"

	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
)

const (
	minimumRenewalUsageRate                 = 0.30
	minimumRenewalPriceRate                 = 0.30
	subscriptionRenewalQuotaThresholdReason = "renewal requires at least 30% of the current package quota to be used"
)

func subscriptionUsageRate(sub *commerceschema.UserSubscription) float64 {
	if sub == nil || sub.AmountTotal <= 0 {
		return 1
	}
	usedRate := float64(sub.AmountUsed) / float64(sub.AmountTotal)
	return math.Max(0, math.Min(usedRate, 1))
}

func isSubscriptionRenewalEligible(sub *commerceschema.UserSubscription) bool {
	return subscriptionUsageRate(sub) >= minimumRenewalUsageRate
}

func calculateRenewalPrice(plan *commerceschema.SubscriptionPlan, sub *commerceschema.UserSubscription) float64 {
	if plan == nil || plan.PriceAmount <= 0 {
		return 0
	}
	if sub == nil || sub.AmountTotal <= 0 {
		return plan.PriceAmount
	}

	priceRate := math.Max(subscriptionUsageRate(sub), minimumRenewalPriceRate)
	return math.Round(plan.PriceAmount*priceRate*100) / 100
}
