package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	stdhttp "net/http"
)

func TestQuoteSubscriptionFuelReturnsPlanSpecificQuote(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	user := &identityschema.User{
		Id: 1, Username: "subscription-fuel", Password: "password123",
		DisplayName: "Subscription Fuel", Role: constant.RoleCommonUser,
		Status: constant.UserStatusEnabled, Group: "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	plan := &commerceschema.SubscriptionPlan{
		Id: 1, Title: "Standard月卡", Enabled: true, PriceAmount: 89,
		DurationUnit: commerceschema.SubscriptionDurationMonth, DurationValue: 1,
		PlanType: commerceschema.SubscriptionPlanTypeMonthly, FuelEnabled: true,
		FuelUnitPrice: 0.17, FuelMinQuota: 5_000_000, FuelQuotaStep: 5_000_000,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	subscription := &commerceschema.UserSubscription{
		Id: 1, UserId: user.Id, PlanId: plan.Id, AmountTotal: 300_000_000,
		StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(), Status: "active",
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to seed user subscription: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodPost, "/api/subscription/fuel/quote", map[string]any{
		"subscription_id": subscription.Id,
		"quota":           10_000_000,
	}, user.Id)
	quoteSubscriptionFuel(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected fuel quote success, got %#v", response)
	}
	var payload struct {
		SubscriptionID int     `json:"subscription_id"`
		UnitPrice      float64 `json:"unit_price"`
		AmountDue      float64 `json:"amount_due"`
		MinQuota       int64   `json:"min_quota"`
		QuotaStep      int64   `json:"quota_step"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode fuel quote: %v", err)
	}
	if payload.SubscriptionID != subscription.Id || payload.UnitPrice != 0.17 || payload.AmountDue != 3.4 ||
		payload.MinQuota != plan.FuelMinQuota || payload.QuotaStep != plan.FuelQuotaStep {
		t.Fatalf("unexpected fuel quote: %#v", payload)
	}
}

func TestQuoteSubscriptionFuelRejectsInvalidRequest(t *testing.T) {
	setupCommerceHTTPTestDB(t)
	ctx, recorder := newCommerceContext(t, stdhttp.MethodPost, "/api/subscription/fuel/quote", map[string]any{
		"subscription_id": "invalid",
	}, 1)
	quoteSubscriptionFuel(ctx)

	response := decodeCommerceResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected invalid fuel quote request to fail, got %#v", response)
	}
}

func TestGetSubscriptionPlansReturnsPublicPlans(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	confirmTopupComplianceForTest(t)

	if err := db.Create(&commerceschema.SubscriptionPlan{
		Id:            1,
		Title:         "Standard月卡",
		Enabled:       true,
		InternalOnly:  false,
		PriceAmount:   30,
		SortOrder:     10,
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodGet, "/api/subscription/plans", nil, 0)
	getSubscriptionPlans(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected subscription plans success, got %#v", response)
	}

	var payload []map[string]json.RawMessage
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode plans payload: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(payload))
	}
}

func TestGetSubscriptionSelfReturnsOverview(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "subscription-self",
		Password:    "password123",
		DisplayName: "Subscription Self",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	identitydomain.SetSetting(user, dto.UserSetting{
		BillingPreference:  "subscription_first",
		FundingSourceOrder: []string{"subscription", "wallet"},
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodGet, "/api/subscription/self", nil, user.Id)
	getSubscriptionSelf(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected subscription self success, got %#v", response)
	}

	var payload struct {
		BillingPreference string `json:"billing_preference"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode self payload: %v", err)
	}
	if payload.BillingPreference != "subscription_first" {
		t.Fatalf("expected subscription_first preference, got %q", payload.BillingPreference)
	}
}

func TestUpdateSubscriptionPreferencePersistsSetting(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "subscription-pref",
		Password:    "password123",
		DisplayName: "Subscription Preference",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodPut, "/api/subscription/self/preference", map[string]any{
		"billing_preference":     "wallet_first",
		"funding_source_order":   []string{"wallet", "subscription"},
		"subscription_order_ids": []int{2, 1},
	}, user.Id)
	updateSubscriptionPreference(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update subscription preference success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	setting := identitydomain.GetSetting(reloaded)
	if setting.BillingPreference != "wallet_first" {
		t.Fatalf("expected wallet_first billing preference, got %q", setting.BillingPreference)
	}
	if len(setting.SubscriptionOrderIds) != 2 || setting.SubscriptionOrderIds[0] != 2 {
		t.Fatalf("unexpected subscription order ids: %#v", setting.SubscriptionOrderIds)
	}
}

func TestGetSubscriptionOrderStatusReturnsOrderPayload(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	commerceapp.InvalidateSubscriptionPlanCache(1)
	user := &identityschema.User{
		Id:          1,
		Username:    "subscription-order",
		Password:    "password123",
		DisplayName: "Subscription Order",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&commerceschema.SubscriptionPlan{
		Id:            1,
		Title:         "Pro月卡",
		Enabled:       true,
		PriceAmount:   60,
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	order := &commerceschema.SubscriptionOrder{
		Id:              1,
		UserId:          user.Id,
		PlanId:          1,
		Money:           60,
		TradeNo:         "sub-order-1",
		PaymentMethod:   commerceschema.PaymentMethodStripe,
		PaymentProvider: commerceschema.PaymentProviderStripe,
		Status:          constant.TopUpStatusSuccess,
		CreateTime:      platformruntime.GetTimestamp(),
		CompleteTime:    platformruntime.GetTimestamp(),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to seed subscription order: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodGet, "/api/subscription/orders/sub-order-1", nil, user.Id)
	ctx.Params = gin.Params{{Key: "trade_no", Value: order.TradeNo}}
	getSubscriptionOrderStatus(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected order status success, got %#v", response)
	}

	var payload struct {
		TradeNo   string `json:"trade_no"`
		PlanTitle string `json:"plan_title"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode order payload: %v", err)
	}
	if payload.TradeNo != order.TradeNo || payload.PlanTitle != "Pro月卡" {
		t.Fatalf("unexpected order payload: %#v", payload)
	}
}
