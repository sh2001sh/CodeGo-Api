package app

// RedemptionResult describes the outcome of one user redemption operation.
type RedemptionResult struct {
	RedeemType         string `json:"redeem_type"`
	Quota              int    `json:"quota,omitempty"`
	PlanId             int    `json:"plan_id,omitempty"`
	PlanTitle          string `json:"plan_title,omitempty"`
	UserSubscriptionId int    `json:"user_subscription_id,omitempty"`
}
