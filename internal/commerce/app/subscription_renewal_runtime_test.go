package app

import (
	"testing"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveSubscriptionPurchasePreview_RenewalRequiresThirtyPercentUsage(t *testing.T) {
	db := setupRedemptionTestDB(t)
	plan := &commerceschema.SubscriptionPlan{
		Id:            9510,
		Title:         "Renewal price plan",
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
		PriceAmount:   100,
		TotalAmount:   1000,
	}
	users := []*identityschema.User{
		{Id: 9511, Username: "renewal_price_usage", Status: constant.UserStatusEnabled},
		{Id: 9512, Username: "renewal_price_floor", Status: constant.UserStatusEnabled},
		{Id: 9515, Username: "renewal_too_early", Status: constant.UserStatusEnabled},
	}
	records := []*commerceschema.UserSubscription{
		{Id: 9513, UserId: 9511, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 600, EndTime: time.Now().Add(time.Hour).Unix(), Status: "active"},
		{Id: 9514, UserId: 9512, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 300, EndTime: time.Now().Add(time.Hour).Unix(), Status: "active"},
		{Id: 9516, UserId: 9515, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 299, EndTime: time.Now().Add(time.Hour).Unix(), Status: "active"},
	}
	require.NoError(t, db.Create(plan).Error)
	for _, user := range users {
		require.NoError(t, db.Create(user).Error)
	}
	for _, record := range records {
		require.NoError(t, db.Create(record).Error)
	}

	usagePreview, err := ResolveSubscriptionPurchasePreview(9511, plan)
	require.NoError(t, err)
	assert.Equal(t, commerceschema.SubscriptionPurchaseActionRenew, usagePreview.Action)
	assert.Equal(t, 60.0, usagePreview.AmountDue)

	floorPreview, err := ResolveSubscriptionPurchasePreview(9512, plan)
	require.NoError(t, err)
	assert.Equal(t, commerceschema.SubscriptionPurchaseActionRenew, floorPreview.Action)
	assert.Equal(t, 30.0, floorPreview.AmountDue)

	blockedPreview, err := ResolveSubscriptionPurchasePreview(9515, plan)
	require.NoError(t, err)
	assert.Equal(t, commerceschema.SubscriptionPurchaseActionDisabled, blockedPreview.Action)
	assert.Equal(t, subscriptionRenewalQuotaThresholdReason, blockedPreview.DisabledReason)
	assert.Zero(t, blockedPreview.AmountDue)

	assert.Equal(t, 100.0, calculateRenewalPrice(plan, &commerceschema.UserSubscription{}))
}

func TestRenewUserSubscriptionWithPlanTx_RestartsTermAndRefreshesQuota(t *testing.T) {
	db := setupRedemptionTestDB(t)
	plan := &commerceschema.SubscriptionPlan{
		Id:            9520,
		Title:         "Renewal reset plan",
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
		PeriodAmount:  400,
		ModelLimits:   `{"gpt-test":400}`,
	}
	user := &identityschema.User{Id: 9521, Username: "renewal_reset", Status: constant.UserStatusEnabled}
	oldStart := time.Now().Add(-15 * 24 * time.Hour).Unix()
	subscription := &commerceschema.UserSubscription{
		Id:           9522,
		UserId:       user.Id,
		PlanId:       plan.Id,
		AmountTotal:  1000,
		AmountUsed:   0,
		PeriodAmount: 400,
		PeriodUsed:   0,
		ModelLimits:  plan.ModelLimits,
		ModelUsage:   "",
		StartTime:    oldStart,
		EndTime:      time.Now().Add(15 * 24 * time.Hour).Unix(),
		Status:       "active",
	}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(subscription).Error)
	ensureSubscriptionPreConsumeRecordSchema(t)
	_, err := PreConsumeUserSubscription("renewal-ledger-old-cycle", user.Id, "gpt-other", 300)
	require.NoError(t, err)
	require.NoError(t, SettleSubscriptionReservation("renewal-ledger-old-cycle", subscription.Id, "gpt-other", 300))

	beforeRenewal := time.Now().Unix()
	var renewed *commerceschema.UserSubscription
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		locked := &commerceschema.UserSubscription{}
		if err := tx.First(locked, subscription.Id).Error; err != nil {
			return err
		}
		var err error
		renewed, err = renewUserSubscriptionWithPlanTx(tx, locked, plan, "order")
		return err
	}))

	require.NotNil(t, renewed)
	assert.GreaterOrEqual(t, renewed.StartTime, beforeRenewal)
	assert.Greater(t, renewed.EndTime, subscription.EndTime)
	assert.Equal(t, plan.TotalAmount, renewed.AmountTotal)
	assert.Zero(t, renewed.AmountUsed)
	assert.Equal(t, plan.PeriodAmount, renewed.PeriodAmount)
	assert.Zero(t, renewed.PeriodUsed)
	assert.Empty(t, renewed.ModelUsage)

	var account billingschema.BillingAccount
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ?", "user_subscription", subscription.Id).First(&account).Error)
	var snapshot billingschema.BillingBalanceSnapshot
	require.NoError(t, db.Where("account_id = ?", account.AccountID).First(&snapshot).Error)
	assert.Equal(t, plan.TotalAmount, snapshot.AvailableBalance)
}
