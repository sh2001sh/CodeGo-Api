package app

import (
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"strings"

	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

// SubscriptionPlanDTO is the public subscription plan payload returned to commerce clients.
type SubscriptionPlanDTO struct {
	Plan           commerceschema.SubscriptionPlan `json:"plan"`
	Action         string                          `json:"action,omitempty"`
	BaseAmountDue  float64                         `json:"base_amount_due,omitempty"`
	AmountDue      float64                         `json:"amount_due,omitempty"`
	DisabledReason string                          `json:"disabled_reason,omitempty"`
}

// UpdateSubscriptionPreferenceRequest captures user billing and subscription ordering preferences.
type UpdateSubscriptionPreferenceRequest struct {
	BillingPreference    string   `json:"billing_preference"`
	FundingSourceOrder   []string `json:"funding_source_order"`
	SubscriptionOrderIds []int    `json:"subscription_order_ids"`
}

// ListSubscriptionPlans returns enabled public subscription plans with purchase previews when available.
func ListSubscriptionPlans(userID int) ([]SubscriptionPlanDTO, error) {
	if !IsPaymentComplianceConfirmed() {
		return []SubscriptionPlanDTO{}, nil
	}

	var plans []commerceschema.SubscriptionPlan
	if err := platformdb.DB.Where("enabled = ? AND internal_only = ?", true, false).
		Order("sort_order desc, id desc").
		Find(&plans).Error; err != nil {
		return nil, err
	}

	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, plan := range plans {
		record := SubscriptionPlanDTO{Plan: plan}
		if userID > 0 {
			preview, err := ResolveSubscriptionPurchasePreview(userID, &plan)
			if err == nil && preview != nil {
				record.Action = preview.Action
				record.BaseAmountDue = preview.BaseAmountDue
				record.AmountDue = preview.AmountDue
				record.DisabledReason = preview.DisabledReason
			}
		}
		result = append(result, record)
	}
	return result, nil
}

// BuildSubscriptionOrderStatusPayload returns a user's subscription order status by trade number.
func BuildSubscriptionOrderStatusPayload(userID int, tradeNo string) (map[string]any, error) {
	order, err := GetSubscriptionOrderByTradeNoForUser(strings.TrimSpace(tradeNo), userID)
	if err != nil {
		return nil, err
	}
	planTitle := ""
	if plan, planErr := GetSubscriptionPlanByID(order.PlanId); planErr == nil && plan != nil {
		planTitle = plan.Title
	}
	return map[string]any{
		"trade_no":         order.TradeNo,
		"status":           order.Status,
		"plan_id":          order.PlanId,
		"plan_title":       planTitle,
		"money":            order.Money,
		"payment_method":   order.PaymentMethod,
		"payment_provider": order.PaymentProvider,
		"create_time":      order.CreateTime,
		"complete_time":    order.CompleteTime,
	}, nil
}

// BuildSubscriptionSelfPayload returns the user's subscription overview and preference state.
func BuildSubscriptionSelfPayload(userID int) (map[string]any, error) {
	settingMap, _ := identitystore.LoadUserSetting(userID, false)
	preference := commercedomain.NormalizeBillingPreference(settingMap.BillingPreference)
	fundingSourceOrder := commercedomain.NormalizeFundingSourceOrder(settingMap.FundingSourceOrder, preference)
	preference = commercedomain.BillingPreferenceFromFundingSourceOrder(fundingSourceOrder)

	allSubscriptions, err := GetAllUserSubscriptions(userID)
	if err != nil {
		allSubscriptions = []commercedomain.SubscriptionSummary{}
	}
	activeSubscriptions, err := GetAllActiveUserSubscriptions(userID)
	if err != nil {
		activeSubscriptions = []commercedomain.SubscriptionSummary{}
	}

	activeSubscriptionIDs := make([]int, 0, len(activeSubscriptions))
	activeSubscriptionSet := make(map[int]struct{}, len(activeSubscriptions))
	for _, item := range activeSubscriptions {
		if item.Subscription == nil || item.Subscription.Id <= 0 {
			continue
		}
		activeSubscriptionIDs = append(activeSubscriptionIDs, item.Subscription.Id)
		activeSubscriptionSet[item.Subscription.Id] = struct{}{}
	}

	orderedIDs := make([]int, 0, len(activeSubscriptionIDs))
	for _, id := range commercedomain.NormalizePositiveIntSlice(settingMap.SubscriptionOrderIds) {
		if _, ok := activeSubscriptionSet[id]; !ok {
			continue
		}
		orderedIDs = append(orderedIDs, id)
		delete(activeSubscriptionSet, id)
	}
	for _, id := range activeSubscriptionIDs {
		if _, ok := activeSubscriptionSet[id]; !ok {
			continue
		}
		orderedIDs = append(orderedIDs, id)
		delete(activeSubscriptionSet, id)
	}

	return map[string]any{
		"billing_preference":     preference,
		"funding_source_order":   fundingSourceOrder,
		"subscription_order_ids": orderedIDs,
		"subscriptions":          activeSubscriptions,
		"all_subscriptions":      allSubscriptions,
	}, nil
}

// UpdateSubscriptionPreference persists the user's billing preference and active subscription ordering.
func UpdateSubscriptionPreference(userID int, req UpdateSubscriptionPreferenceRequest) (map[string]any, error) {
	preference := commercedomain.NormalizeBillingPreference(req.BillingPreference)
	fundingSourceOrder := commercedomain.NormalizeFundingSourceOrder(req.FundingSourceOrder, preference)
	preference = commercedomain.BillingPreferenceFromFundingSourceOrder(fundingSourceOrder)
	orderIDs := commercedomain.NormalizePositiveIntSlice(req.SubscriptionOrderIds)

	user, err := loadCommerceUserByID(userID, true)
	if err != nil {
		return nil, err
	}
	current := identitydomain.GetSetting(user)
	current.BillingPreference = preference
	current.FundingSourceOrder = fundingSourceOrder
	if req.SubscriptionOrderIds != nil {
		current.SubscriptionOrderIds = orderIDs
	}
	identitydomain.SetSetting(user, current)
	if err := identitystore.UpdateUser(user, false); err != nil {
		return nil, err
	}

	return map[string]any{
		"billing_preference":     preference,
		"funding_source_order":   current.FundingSourceOrder,
		"subscription_order_ids": current.SubscriptionOrderIds,
	}, nil
}
