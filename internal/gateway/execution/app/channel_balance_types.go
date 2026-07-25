package app

type openAISubscriptionResponse struct {
	HasPaymentMethod bool    `json:"has_payment_method"`
	HardLimitUSD     float64 `json:"hard_limit_usd"`
}

type openAIUsageResponse struct {
	TotalUsage float64 `json:"total_usage"`
}

type aiProxyUserOverviewResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode int    `json:"error_code"`
	Data      struct {
		TotalPoints float64 `json:"totalPoints"`
	} `json:"data"`
}

type api2GPTUsageResponse struct {
	TotalRemaining float64 `json:"total_remaining"`
}

type aigc2DUsageResponse struct {
	TotalAvailable float64 `json:"total_available"`
}

type siliconFlowUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TotalBalance string `json:"totalBalance"`
	} `json:"data"`
}

type deepSeekUsageResponse struct {
	BalanceInfos []struct {
		Currency     string `json:"currency"`
		TotalBalance string `json:"total_balance"`
	} `json:"balance_infos"`
}

type openRouterCreditResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}
