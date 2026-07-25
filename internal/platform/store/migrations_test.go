package store

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sh2001sh/CodeGo-Api/constant"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type legacyUserForExternalIDMigration struct {
	Id        int `gorm:"primaryKey"`
	Username  string
	DeletedAt gorm.DeletedAt
}

func (legacyUserForExternalIDMigration) TableName() string {
	return "users"
}

func TestApplyV2MigrationsIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
	})
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false
	require.NoError(t, ApplyV2Migrations(context.Background(), false))
	require.NoError(t, ApplyV2Migrations(context.Background(), false))

	var migrationCount int64
	require.NoError(t, db.Model(&schemaMigration{}).Count(&migrationCount).Error)
	require.Equal(t, int64(len(V2MigrationIDs())), migrationCount)
	for _, table := range []string{
		"billing_outbox_events",
		"workflow_task_workflows",
		"workflow_task_snapshots",
		"workflow_task_terminal_results",
		"subscription_plans",
		"subscription_orders",
		"user_subscriptions",
		"subscription_pre_consume_records",
		"gateway_request_executions",
		"gateway_route_plans",
		"gateway_execution_attempts",
		"gateway_usage_evidence",
		"route_pools",
		"route_pool_members",
	} {
		require.True(t, db.Migrator().HasTable(table), table)
	}
}

func TestSubscriptionFulfillmentMigrationMarksHistoricalSuccessCompleted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
	})
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&commerceschema.SubscriptionOrder{}))
	require.NoError(t, db.Create(&commerceschema.SubscriptionOrder{Id: 1, UserId: 1, PlanId: 1, Status: constant.TopUpStatusSuccess, TradeNo: "historic-success"}).Error)
	require.NoError(t, db.Create(&commerceschema.SubscriptionOrder{Id: 2, UserId: 2, PlanId: 2, Status: constant.TopUpStatusPending, TradeNo: "historic-pending"}).Error)
	require.NoError(t, db.Model(&commerceschema.SubscriptionOrder{}).Where("id IN ?", []int{1, 2}).Update("fulfillment_status", "").Error)

	// Mark the preceding migration steps as already applied to simulate a
	// production database upgraded from the prior v2 revision.
	require.NoError(t, db.AutoMigrate(&schemaMigration{}))
	for _, migrationID := range []string{"20260710_billing_core", "20260710_workflow_core", "20260711_subscription_core", "20260711_gateway_execution_core"} {
		require.NoError(t, db.Create(&schemaMigration{ID: migrationID}).Error)
	}

	require.NoError(t, ApplyV2Migrations(context.Background(), false))
	var completed commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("id = ?", 1).First(&completed).Error)
	require.Equal(t, commerceschema.SubscriptionOrderFulfillmentCompleted, completed.FulfillmentStatus)
	var pending commerceschema.SubscriptionOrder
	require.NoError(t, db.Where("id = ?", 2).First(&pending).Error)
	require.Empty(t, pending.FulfillmentStatus)
}

func TestBootstrapPrimarySchemaThenApplyV2Migrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
	})
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false

	require.NoError(t, BootstrapPrimarySchema())
	require.NoError(t, ApplyV2Migrations(context.Background(), false))
	require.NoError(t, ApplyV2Migrations(context.Background(), false))
	for _, table := range []string{
		"subscription_plans",
		"subscription_orders",
		"user_subscriptions",
		"subscription_pre_consume_records",
		"gateway_request_executions",
	} {
		require.True(t, db.Migrator().HasTable(table), table)
	}
}

func TestMigrateUserExternalIDsBackfillsExistingUsers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyUserForExternalIDMigration{}))
	require.NoError(t, db.Create(&legacyUserForExternalIDMigration{Id: 1, Username: "legacy-one"}).Error)
	require.NoError(t, db.Create(&legacyUserForExternalIDMigration{Id: 2, Username: "legacy-two"}).Error)

	require.NoError(t, migrateUserExternalIDs(db))
	require.True(t, db.Migrator().HasColumn(&identityschema.User{}, "ExternalId"))
	require.True(t, db.Migrator().HasIndex(&identityschema.User{}, "idx_users_external_id"))

	var users []identityschema.User
	require.NoError(t, db.Order("id asc").Find(&users).Error)
	require.Len(t, users, 2)
	require.Len(t, users[0].ExternalId, identityschema.ExternalUserIDLength)
	require.Len(t, users[1].ExternalId, identityschema.ExternalUserIDLength)
	require.NotEqual(t, users[0].ExternalId, users[1].ExternalId)

	firstID := users[0].ExternalId
	require.NoError(t, migrateUserExternalIDs(db))
	require.NoError(t, db.First(&users[0], 1).Error)
	require.Equal(t, firstID, users[0].ExternalId)
}
