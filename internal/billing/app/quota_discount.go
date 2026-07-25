package app

import "math"

func getEffectiveConsumptionDiscountRate(userID int) float64 {
	return 0
}

func applyUsageConsumptionDiscount(userID int, quota int) int {
	if quota <= 0 {
		return quota
	}
	rate := getEffectiveConsumptionDiscountRate(userID)
	if rate <= 0 {
		return quota
	}
	return int(math.Round(float64(quota) * (1 - rate)))
}
