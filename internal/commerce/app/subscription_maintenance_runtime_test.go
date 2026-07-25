package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/store"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestResetDueSubscriptions_ResetsPeriodUsageAndModelUsage(t *testing.T) {
	db := setupRedemptionTestDB(t)

	plan := insertSubscriptionResetAppTestPlan(t, 9901, 0, int64(platformruntime.QuotaPerUnit)*20)
	plan.PeriodAmount = int64(platformruntime.QuotaPerUnit) * 4
	plan.QuotaResetPeriod = commerceschema.SubscriptionResetDaily
	require.NoError(t, db.Save(plan).Error)
	modelUsage, err := commercedomain.EncodeSubscriptionModelQuotaMap(map[string]int64{
		"gpt-4.1": int64(platformruntime.QuotaPerUnit / 2),
	})
	require.NoError(t, err)

	now := time.Now().Unix()
	sub := &commerceschema.UserSubscription{
		Id:            9902,
		UserId:        9901,
		PlanId:        plan.Id,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    int64(platformruntime.QuotaPerUnit) * 2,
		PeriodAmount:  plan.PeriodAmount,
		PeriodUsed:    int64(platformruntime.QuotaPerUnit),
		ModelUsage:    modelUsage,
		StartTime:     now - 3*86400,
		EndTime:       now + 30*86400,
		Status:        "active",
		LastResetTime: now - 2*86400,
		NextResetTime: now - 3600,
	}
	require.NoError(t, db.Create(sub).Error)

	resetCount, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, resetCount)

	var reloaded commerceschema.UserSubscription
	require.NoError(t, db.Where("id = ?", sub.Id).First(&reloaded).Error)
	assert.EqualValues(t, int64(platformruntime.QuotaPerUnit)*2, reloaded.AmountUsed)
	assert.Zero(t, reloaded.PeriodUsed)
	assert.Equal(t, "", reloaded.ModelUsage)
	assert.Greater(t, reloaded.LastResetTime, sub.LastResetTime)
	assert.Greater(t, reloaded.NextResetTime, now)
}

func TestCleanupSubscriptionPreConsumeRecords_RemovesExpiredRows(t *testing.T) {
	db := setupRedemptionTestDB(t)
	ensureSubscriptionPreConsumeRecordSchema(t)

	oldRecord := &commerceschema.SubscriptionPreConsumeRecord{
		RequestId:          "cleanup-old",
		UserId:             1,
		UserSubscriptionId: 2,
		PreConsumed:        10,
		Status:             "consumed",
	}
	newRecord := &commerceschema.SubscriptionPreConsumeRecord{
		RequestId:          "cleanup-new",
		UserId:             1,
		UserSubscriptionId: 3,
		PreConsumed:        20,
		Status:             "consumed",
	}
	require.NoError(t, db.Create(oldRecord).Error)
	require.NoError(t, db.Create(newRecord).Error)
	require.NoError(t, db.Model(&commerceschema.SubscriptionPreConsumeRecord{}).
		Where("id = ?", oldRecord.Id).
		Update("updated_at", commercestore.GetDBTimestamp()-8*24*3600).Error)

	rows, err := CleanupSubscriptionPreConsumeRecords(7 * 24 * 3600)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	var remaining []commerceschema.SubscriptionPreConsumeRecord
	require.NoError(t, db.Order("id asc").Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, "cleanup-new", remaining[0].RequestId)
}

func TestExpireDueSubscriptions_DowngradesUserGroupWhenLastUpgradeExpires(t *testing.T) {
	db := setupRedemptionTestDB(t)

	user := &identityschema.User{
		Id:       9950,
		Username: "expire_due_group_user",
		Status:   constant.UserStatusEnabled,
		Group:    "vip",
	}
	require.NoError(t, db.Create(user).Error)

	now := time.Now().Unix()
	sub := &commerceschema.UserSubscription{
		Id:            9951,
		UserId:        user.Id,
		PlanId:        9952,
		Status:        "active",
		StartTime:     now - 30*24*3600,
		EndTime:       now - 60,
		UpgradeGroup:  "vip",
		PrevUserGroup: "default",
	}
	require.NoError(t, db.Create(sub).Error)

	expiredCount, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, expiredCount)

	var savedSub commerceschema.UserSubscription
	require.NoError(t, db.Where("id = ?", sub.Id).First(&savedSub).Error)
	assert.Equal(t, "expired", savedSub.Status)

	var savedUser identityschema.User
	require.NoError(t, db.Where("id = ?", user.Id).First(&savedUser).Error)
	assert.Equal(t, "default", savedUser.Group)
}
