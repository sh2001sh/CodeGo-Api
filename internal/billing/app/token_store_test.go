package app

import (
	"testing"

	"github.com/glebarez/sqlite"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAdjustTokenQuotaUsesLedgerAndProjectsLegacyFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	originalRedisEnabled := platformcache.RedisEnabled
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
		platformcache.RedisEnabled = originalRedisEnabled
	})
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false
	platformcache.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&identityschema.Token{},
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
	))
	token := &identityschema.Token{Id: 901, UserId: 7, Key: "ledger-token", RemainQuota: 1_000}
	require.NoError(t, db.Create(token).Error)

	require.NoError(t, AdjustTokenQuota(token.Id, token.Key, 250))
	loaded, err := GetTokenByID(token.Id)
	require.NoError(t, err)
	require.Equal(t, 750, loaded.RemainQuota)

	var legacy identityschema.Token
	require.NoError(t, db.First(&legacy, token.Id).Error)
	require.Equal(t, 750, legacy.RemainQuota)
	require.Equal(t, 250, legacy.UsedQuota)

	var account billingschema.BillingAccount
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND account_type = ?", "token", token.Id, "token").First(&account).Error)
	var snapshot billingschema.BillingBalanceSnapshot
	require.NoError(t, db.Where("account_id = ?", account.AccountID).First(&snapshot).Error)
	require.EqualValues(t, 750, snapshot.AvailableBalance)
}
