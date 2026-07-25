package app

import (
	"github.com/glebarez/sqlite"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func setupRedemptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	platformdb.UsingSQLite = true
	platformdb.UsingMySQL = false
	platformdb.UsingPostgreSQL = false
	platformcache.RedisEnabled = false
	platformconfig.BatchUpdateEnabled = false
	platformconfig.LogConsumeEnabled = true

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	platformdb.DB = db
	platformdb.LogDB = db

	require.NoError(t, db.AutoMigrate(
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
		&identityschema.User{},
		&auditschema.Log{},
		&commerceschema.Redemption{},
		&commerceschema.TopUp{},
		&commerceschema.SubscriptionPlan{},
		&commerceschema.SubscriptionOrder{},
		&commerceschema.UserSubscription{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestRedeemCodeReturnsSpecificBusinessErrors(t *testing.T) {
	db := setupRedemptionTestDB(t)

	user := &identityschema.User{Id: 8810, Username: "redeem_error_user", Status: constant.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	tests := []struct {
		name       string
		key        string
		redemption commerceschema.Redemption
		wantErr    error
	}{
		{
			name: "invalid code",
			key:  "missing-code",
			redemption: commerceschema.Redemption{
				Id:          9910,
				Key:         "invalid-code",
				Name:        "Invalid",
				Status:      constant.RedemptionCodeStatusEnabled,
				CreatedTime: time.Now().Unix(),
			},
			wantErr: commercedomain.ErrRedemptionInvalid,
		},
		{
			name: "used code",
			redemption: commerceschema.Redemption{
				Id:           9911,
				Key:          "used-code",
				Name:         "Used",
				Status:       constant.RedemptionCodeStatusUsed,
				CreatedTime:  time.Now().Unix(),
				RedeemedTime: time.Now().Unix(),
				UsedUserId:   user.Id,
			},
			wantErr: commercedomain.ErrRedemptionUsed,
		},
		{
			name: "expired code",
			redemption: commerceschema.Redemption{
				Id:          9912,
				Key:         "expired-code",
				Name:        "Expired",
				Status:      constant.RedemptionCodeStatusEnabled,
				ExpiredTime: time.Now().Add(-time.Hour).Unix(),
				CreatedTime: time.Now().Unix(),
			},
			wantErr: commercedomain.ErrRedemptionExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, db.Create(&tt.redemption).Error)
			redeemKey := tt.redemption.Key
			if tt.key != "" {
				redeemKey = tt.key
			}
			_, err := RedeemCode(user.Id, redeemKey)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
