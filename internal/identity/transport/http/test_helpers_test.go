package http

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	auditdomain "github.com/sh2001sh/CodeGo-Api/internal/audit/domain"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

func setupIdentityHTTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	platformdb.UsingSQLite = true
	platformdb.UsingMySQL = false
	platformdb.UsingPostgreSQL = false
	platformcache.RedisEnabled = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	platformdb.DB = db
	platformdb.LogDB = db

	if err := db.AutoMigrate(
		&identityschema.User{},
		&identityschema.Token{},
		&auditschema.Log{},
		&auditdomain.QuotaData{},
		&gatewayschema.Ability{},
		&gatewayschema.Channel{},
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
		&identitydomain.ImageWorkspaceItem{},
		&identitydomain.TwoFA{},
		&identitydomain.TwoFABackupCode{},
		&identitydomain.PasskeyCredential{},
		&identitydomain.CustomOAuthProvider{},
		&identitydomain.UserOAuthBinding{},
		&commerceschema.SubscriptionPlan{},
		&commerceschema.UserSubscription{},
	); err != nil {
		t.Fatalf("failed to migrate identity HTTP tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
