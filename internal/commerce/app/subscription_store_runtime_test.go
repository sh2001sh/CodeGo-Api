package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func insertSubscriptionStoreTestUser(t *testing.T, id int, orderIDs []int) {
	t.Helper()

	user := &identityschema.User{
		Id:       id,
		Username: "subscription_store_test_user",
		Status:   constant.UserStatusEnabled,
	}
	identitydomain.SetSetting(user, dto.UserSetting{
		BillingPreference:    "subscription_first",
		SubscriptionOrderIds: orderIDs,
	})
	require.NoError(t, platformdb.DB.Create(user).Error)
}

func insertSubscriptionStoreTestOrder(t *testing.T, tradeNo string, userID int, planID int, paymentProvider string) {
	t.Helper()

	order := &commerceschema.SubscriptionOrder{
		UserId:          userID,
		PlanId:          planID,
		Money:           9.99,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentProvider,
		PaymentProvider: paymentProvider,
		Status:          constant.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, platformdb.DB.Create(order).Error)
}

func TestSubscriptionStoreLookupsAndCounts(t *testing.T) {
	db := setupRedemptionTestDB(t)

	insertSubscriptionResetAppTestUser(t, 8601, 0)
	plan := insertSubscriptionResetAppTestPlan(t, 8602, 0, int64(platformruntime.QuotaPerUnit)*10)
	insertSubscriptionStoreTestOrder(t, "subscription-store-order", 8601, plan.Id, commerceschema.PaymentProviderStripe)

	now := time.Now().Unix()
	require.NoError(t, db.Create(&commerceschema.UserSubscription{
		Id:          8603,
		UserId:      8601,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 86400,
		Status:      "active",
	}).Error)
	require.NoError(t, db.Create(&commerceschema.UserSubscription{
		Id:          8604,
		UserId:      8601,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 7200,
		EndTime:     now - 3600,
		Status:      "expired",
	}).Error)

	order := GetSubscriptionOrderByTradeNo("subscription-store-order")
	require.NotNil(t, order)
	assert.Equal(t, 8601, order.UserId)

	userOrder, err := GetSubscriptionOrderByTradeNoForUser("subscription-store-order", 8601)
	require.NoError(t, err)
	assert.Equal(t, order.TradeNo, userOrder.TradeNo)

	count, err := CountUserSubscriptionsByPlan(8601, plan.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	allSubs, err := GetAllUserSubscriptions(8601)
	require.NoError(t, err)
	require.Len(t, allSubs, 2)
	assert.Equal(t, 8603, allSubs[0].Subscription.Id)
	assert.Equal(t, 8604, allSubs[1].Subscription.Id)
}

func TestGetSubscriptionPlanInfoByUserSubscriptionID(t *testing.T) {
	db := setupRedemptionTestDB(t)

	insertSubscriptionResetAppTestUser(t, 8611, 0)
	plan := insertSubscriptionResetAppTestPlan(t, 8612, 0, int64(platformruntime.QuotaPerUnit)*10)
	now := time.Now().Unix()
	sub := &commerceschema.UserSubscription{
		Id:          8613,
		UserId:      8611,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 86400,
		Status:      "active",
	}
	require.NoError(t, db.Create(sub).Error)

	info, err := GetSubscriptionPlanInfoByUserSubscriptionID(sub.Id)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, plan.Id, info.PlanId)
	assert.Equal(t, plan.Title, info.PlanTitle)
}

func TestGetAllActiveUserSubscriptions_KeepsExhaustedDayPassVisibleAndSkipsBilling(t *testing.T) {
	db := setupRedemptionTestDB(t)
	require.NoError(t, db.AutoMigrate(&commerceschema.SubscriptionPreConsumeRecord{}))

	now := time.Now().Unix()
	dayQuota := int64(platformruntime.QuotaPerUnit)
	monthQuota := int64(platformruntime.QuotaPerUnit) * 10

	dayPlan := insertSubscriptionResetAppTestPlan(t, 9301, 1, dayQuota)
	monthPlan := insertSubscriptionResetAppTestPlan(t, 9302, 0, monthQuota)

	daySub := &commerceschema.UserSubscription{
		Id:          9401,
		UserId:      9301,
		PlanId:      dayPlan.Id,
		AmountTotal: dayPlan.TotalAmount,
		AmountUsed:  dayPlan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 86400,
		Status:      "active",
	}
	monthSub := &commerceschema.UserSubscription{
		Id:          9402,
		UserId:      9301,
		PlanId:      monthPlan.Id,
		AmountTotal: monthPlan.TotalAmount,
		StartTime:   now - 3600,
		EndTime:     now + 30*86400,
		Status:      "active",
	}

	insertSubscriptionStoreTestUser(t, 9301, []int{daySub.Id, monthSub.Id})
	require.NoError(t, db.Create(daySub).Error)
	require.NoError(t, db.Create(monthSub).Error)

	activeSubs, err := GetAllActiveUserSubscriptions(9301)
	require.NoError(t, err)
	require.Len(t, activeSubs, 2)
	assert.Equal(t, daySub.Id, activeSubs[0].Subscription.Id)
	assert.Equal(t, monthSub.Id, activeSubs[1].Subscription.Id)

	result, err := PreConsumeUserSubscription("subscription-ordering-req", 9301, "", int64(platformruntime.QuotaPerUnit))
	require.NoError(t, err)
	assert.Equal(t, monthSub.Id, result.UserSubscriptionId)
	assert.EqualValues(t, platformruntime.QuotaPerUnit, result.PreConsumed)

	var reloadedDay commerceschema.UserSubscription
	require.NoError(t, db.Where("id = ?", daySub.Id).First(&reloadedDay).Error)
	assert.Equal(t, dayPlan.TotalAmount, reloadedDay.AmountUsed)
	assert.Equal(t, "active", reloadedDay.Status)

	var reloadedMonth commerceschema.UserSubscription
	require.NoError(t, db.Where("id = ?", monthSub.Id).First(&reloadedMonth).Error)
	assert.EqualValues(t, platformruntime.QuotaPerUnit, reloadedMonth.AmountUsed)
}

func TestCompleteSubscriptionOrder_RejectsMismatchedPaymentProvider(t *testing.T) {
	db := setupRedemptionTestDB(t)

	insertSubscriptionResetAppTestUser(t, 202, 0)
	plan := insertSubscriptionResetAppTestPlan(t, 301, 0, int64(platformruntime.QuotaPerUnit)*10)
	insertSubscriptionStoreTestOrder(t, "sub-guard-order", 202, plan.Id, commerceschema.PaymentProviderStripe)

	err := CompleteSubscriptionOrder("sub-guard-order", `{"provider":"creem"}`, commerceschema.PaymentProviderCreem, "card")
	require.ErrorIs(t, err, commerceschema.ErrPaymentMethodMismatch)

	var order commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", "sub-guard-order").First(&order).Error)
	assert.Equal(t, constant.TopUpStatusPending, order.Status)

	var subscriptionCount int64
	require.NoError(t, db.Model(&commerceschema.UserSubscription{}).Where("user_id = ?", 202).Count(&subscriptionCount).Error)
	assert.Zero(t, subscriptionCount)

	var topUpCount int64
	require.NoError(t, db.Model(&commerceschema.TopUp{}).Where("trade_no = ?", "sub-guard-order").Count(&topUpCount).Error)
	assert.Zero(t, topUpCount)
}

func TestPaidSubscriptionOrderRequiresWorkflowFulfillment(t *testing.T) {
	db := setupRedemptionTestDB(t)

	insertSubscriptionResetAppTestUser(t, 722, 0)
	plan := insertSubscriptionResetAppTestPlan(t, 723, 0, int64(platformruntime.QuotaPerUnit)*10)
	insertSubscriptionStoreTestOrder(t, "sub-workflow-order", 722, plan.Id, commerceschema.PaymentProviderStripe)

	require.NoError(t, CompleteSubscriptionOrder("sub-workflow-order", `{"paid":true}`, commerceschema.PaymentProviderStripe, "card"))

	var paid commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", "sub-workflow-order").First(&paid).Error)
	assert.Equal(t, constant.TopUpStatusSuccess, paid.Status)
	assert.Equal(t, commerceschema.SubscriptionOrderFulfillmentPending, paid.FulfillmentStatus)

	var beforeCount int64
	require.NoError(t, db.Model(&commerceschema.UserSubscription{}).Where("user_id = ?", paid.UserId).Count(&beforeCount).Error)
	assert.Zero(t, beforeCount)

	require.NoError(t, FulfillPaidSubscriptionOrder(paid.TradeNo))
	require.NoError(t, FulfillPaidSubscriptionOrder(paid.TradeNo))

	var fulfilled commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", paid.TradeNo).First(&fulfilled).Error)
	assert.Equal(t, commerceschema.SubscriptionOrderFulfillmentCompleted, fulfilled.FulfillmentStatus)
	var afterCount int64
	require.NoError(t, db.Model(&commerceschema.UserSubscription{}).Where("user_id = ?", paid.UserId).Count(&afterCount).Error)
	assert.EqualValues(t, 1, afterCount)
}

func TestExpireSubscriptionOrder_RejectsMismatchedPaymentProvider(t *testing.T) {
	db := setupRedemptionTestDB(t)

	insertSubscriptionResetAppTestUser(t, 303, 0)
	plan := insertSubscriptionResetAppTestPlan(t, 401, 0, int64(platformruntime.QuotaPerUnit)*10)
	insertSubscriptionStoreTestOrder(t, "sub-expire-guard", 303, plan.Id, commerceschema.PaymentProviderStripe)

	err := ExpireSubscriptionOrder("sub-expire-guard", commerceschema.PaymentProviderCreem)
	require.ErrorIs(t, err, commerceschema.ErrPaymentMethodMismatch)

	var order commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", "sub-expire-guard").First(&order).Error)
	assert.Equal(t, constant.TopUpStatusPending, order.Status)
}
