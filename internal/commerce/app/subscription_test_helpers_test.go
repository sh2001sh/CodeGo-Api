package app

import (
	"fmt"
	"testing"

	"github.com/sh2001sh/CodeGo-Api/constant"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/stretchr/testify/require"
)

func insertSubscriptionResetAppTestUser(t *testing.T, id int, _ int) {
	t.Helper()
	user := &identityschema.User{
		Id:       id,
		Username: fmt.Sprintf("subscription_test_user_%d", id),
		Status:   constant.UserStatusEnabled,
	}
	require.NoError(t, platformdb.DB.Create(user).Error)
}

func insertSubscriptionResetAppTestPlan(t *testing.T, id int, durationDays int, totalAmount int64) *commerceschema.SubscriptionPlan {
	t.Helper()
	plan := &commerceschema.SubscriptionPlan{
		Id:            id,
		Title:         "Subscription Test Plan",
		PriceAmount:   50,
		Currency:      "CNY",
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   totalAmount,
	}
	if durationDays > 0 {
		plan.DurationUnit = commerceschema.SubscriptionDurationDay
		plan.DurationValue = durationDays
	}
	require.NoError(t, platformdb.DB.Create(plan).Error)
	return plan
}
