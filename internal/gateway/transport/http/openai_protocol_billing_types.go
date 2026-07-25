package http

import (
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

type openAIProtocolSubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type openAIProtocolUsageResponse struct {
	Object     string  `json:"object"`
	TotalUsage float64 `json:"total_usage"`
}

func quotaToOpenAIUSD(quota int) float64 {
	return float64(quota) / platformruntime.QuotaPerUnit
}

func quotaToOpenAIUsage(quota int) float64 {
	return quotaToOpenAIUSD(quota) * 100
}
