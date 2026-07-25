package app

import (
	"testing"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createSubscriptionFuelTestPlan(t *testing.T, id int) *commerceschema.SubscriptionPlan {
	t.Helper()
	plan := insertSubscriptionResetAppTestPlan(t, id, 0, quotaUnitsFromUSD(300))
	plan.Title = "Fuel Test Monthly Plan"
	plan.PlanType = commerceschema.SubscriptionPlanTypeMonthly
	plan.FuelEnabled = true
	plan.FuelUnitPrice = 0.17
	plan.FuelMinQuota = quotaUnitsFromUSD(10)
	plan.FuelQuotaStep = quotaUnitsFromUSD(10)
	require.NoError(t, platformdb.DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	return plan
}

func createSubscriptionFuelTestSubscription(t *testing.T, userID, subscriptionID, planID int, endTime int64) *commerceschema.UserSubscription {
	t.Helper()
	subscription := &commerceschema.UserSubscription{
		Id:          subscriptionID,
		UserId:      userID,
		PlanId:      planID,
		AmountTotal: quotaUnitsFromUSD(300),
		AmountUsed:  quotaUnitsFromUSD(250),
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     endTime,
		Status:      "active",
	}
	require.NoError(t, platformdb.DB.Create(subscription).Error)
	return subscription
}

func TestQuoteSubscriptionFuel_UsesPlanSpecificRules(t *testing.T) {
	setupRedemptionTestDB(t)
	insertSubscriptionStoreTestUser(t, 9811, []int{9812})
	plan := createSubscriptionFuelTestPlan(t, 9813)
	subscription := createSubscriptionFuelTestSubscription(t, 9811, 9812, plan.Id, time.Now().Add(24*time.Hour).Unix())

	quote, err := QuoteSubscriptionFuel(9811, SubscriptionFuelQuoteRequest{
		SubscriptionID: subscription.Id,
		Quota:          quotaUnitsFromUSD(20),
	})
	require.NoError(t, err)
	assert.Equal(t, plan.Id, quote.PlanID)
	assert.Equal(t, plan.FuelUnitPrice, quote.UnitPrice)
	assert.Equal(t, 3.4, quote.AmountDue)
	assert.Equal(t, subscription.EndTime, quote.ExpiresAt)

	_, err = QuoteSubscriptionFuel(9811, SubscriptionFuelQuoteRequest{SubscriptionID: subscription.Id, Quota: quotaUnitsFromUSD(5)})
	require.ErrorIs(t, err, ErrSubscriptionFuelUnavailable)
	_, err = QuoteSubscriptionFuel(9811, SubscriptionFuelQuoteRequest{SubscriptionID: subscription.Id, Quota: quotaUnitsFromUSD(15)})
	require.ErrorIs(t, err, ErrSubscriptionFuelUnavailable)
}

func TestQuoteSubscriptionFuel_RejectsIneligibleSubscriptions(t *testing.T) {
	db := setupRedemptionTestDB(t)
	insertSubscriptionStoreTestUser(t, 9821, []int{9822, 9823, 9824})
	plan := createSubscriptionFuelTestPlan(t, 9825)
	active := createSubscriptionFuelTestSubscription(t, 9821, 9822, plan.Id, time.Now().Add(time.Hour).Unix())

	plan.FuelEnabled = false
	require.NoError(t, db.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	_, err := QuoteSubscriptionFuel(9821, SubscriptionFuelQuoteRequest{SubscriptionID: active.Id, Quota: quotaUnitsFromUSD(10)})
	require.ErrorIs(t, err, ErrSubscriptionFuelUnavailable)

	plan.FuelEnabled = true
	plan.PlanType = commerceschema.SubscriptionPlanTypeDaily
	plan.DurationUnit = commerceschema.SubscriptionDurationDay
	plan.DurationValue = 1
	require.NoError(t, db.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	_, err = QuoteSubscriptionFuel(9821, SubscriptionFuelQuoteRequest{SubscriptionID: active.Id, Quota: quotaUnitsFromUSD(10)})
	require.ErrorIs(t, err, ErrSubscriptionFuelUnavailable)

	plan.PlanType = commerceschema.SubscriptionPlanTypeMonthly
	plan.DurationUnit = commerceschema.SubscriptionDurationMonth
	plan.DurationValue = 1
	require.NoError(t, db.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	expired := createSubscriptionFuelTestSubscription(t, 9821, 9823, plan.Id, time.Now().Add(-time.Minute).Unix())
	_, err = QuoteSubscriptionFuel(9821, SubscriptionFuelQuoteRequest{SubscriptionID: expired.Id, Quota: quotaUnitsFromUSD(10)})
	require.ErrorIs(t, err, ErrSubscriptionFuelUnavailable)
}

func TestFulfillSubscriptionFuel_AddsQuotaOnceWithoutChangingExpiry(t *testing.T) {
	db := setupRedemptionTestDB(t)
	insertSubscriptionStoreTestUser(t, 9831, []int{9832})
	plan := createSubscriptionFuelTestPlan(t, 9833)
	subscription := createSubscriptionFuelTestSubscription(t, 9831, 9832, plan.Id, time.Now().Add(24*time.Hour).Unix())
	order := &commerceschema.SubscriptionOrder{
		UserId:               subscription.UserId,
		PlanId:               plan.Id,
		Money:                3.4,
		TradeNo:              "subscription-fuel-fulfillment",
		PaymentMethod:        "test",
		PaymentProvider:      "test",
		PurchaseType:         commerceschema.SubscriptionPurchaseTypeFuel,
		TargetSubscriptionId: subscription.Id,
		FuelQuota:            quotaUnitsFromUSD(20),
		FuelUnitPrice:        plan.FuelUnitPrice,
		FuelExpiresAt:        subscription.EndTime,
		Status:               constant.TopUpStatusSuccess,
		FulfillmentStatus:    commerceschema.SubscriptionOrderFulfillmentPending,
		CreateTime:           platformruntime.GetTimestamp(),
	}
	require.NoError(t, db.Create(order).Error)

	require.NoError(t, FulfillPaidSubscriptionOrder(order.TradeNo))
	require.NoError(t, FulfillPaidSubscriptionOrder(order.TradeNo))

	var reloadedSubscription commerceschema.UserSubscription
	require.NoError(t, db.Where("id = ?", subscription.Id).First(&reloadedSubscription).Error)
	assert.Equal(t, quotaUnitsFromUSD(320), reloadedSubscription.AmountTotal)
	assert.Equal(t, subscription.EndTime, reloadedSubscription.EndTime)

	var account billingschema.BillingAccount
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND account_type = ?", "user_subscription", subscription.Id, "subscription").First(&account).Error)
	var snapshot billingschema.BillingBalanceSnapshot
	require.NoError(t, db.Where("account_id = ?", account.AccountID).First(&snapshot).Error)
	assert.Equal(t, quotaUnitsFromUSD(20), snapshot.AvailableBalance)

	var reloadedOrder commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", order.TradeNo).First(&reloadedOrder).Error)
	assert.Equal(t, commerceschema.SubscriptionOrderFulfillmentCompleted, reloadedOrder.FulfillmentStatus)
}
