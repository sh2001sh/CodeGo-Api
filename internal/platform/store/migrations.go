package store

import (
	"context"
	"fmt"
	"time"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"gorm.io/gorm"
)

type schemaMigration struct {
	ID        string `gorm:"primaryKey;size:128"`
	AppliedAt time.Time
}

func (schemaMigration) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "platform.schema_migrations"
	}
	return "platform_schema_migrations"
}

type schemaMigrationStep struct {
	ID  string
	Run func(*gorm.DB) error
}

// V2MigrationIDs returns the ordered migration contract required by codego-api.
// Deployment verification uses this list without changing database state.
func V2MigrationIDs() []string {
	return []string{
		"20260710_billing_core",
		"20260710_workflow_core",
		"20260711_subscription_core",
		"20260711_subscription_order_fulfillment",
		"20260711_gateway_execution_core",
		"20260711_gateway_execution_trace",
		"20260714_user_external_id",
		"20260724_gateway_route_pools",
		"20260724_gateway_route_pool_auto_discovery",
	}
}

// ApplyV2Migrations applies only v2-owned tables and records every completed step.
// It deliberately excludes legacy table AutoMigrate calls that are unsafe on old SQLite DDL.
func ApplyV2Migrations(ctx context.Context, dryRun bool) error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	if err := ensureCodeGoSchemas(); err != nil {
		return err
	}
	db := platformdb.DB.WithContext(ctx)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return err
	}

	steps := []schemaMigrationStep{
		{ID: "20260710_billing_core", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&billingschema.BillingAccount{}, &billingschema.BillingBalanceSnapshot{}, &billingschema.BillingLedgerEntry{}, &billingschema.BillingReservation{}, &billingschema.BillingSettlement{}, &billingschema.BillingOutboxEvent{})
		}},
		{ID: "20260710_workflow_core", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&workflowschema.WorkflowTaskWorkflow{}, &workflowschema.WorkflowTaskSnapshot{}, &workflowschema.WorkflowTaskTerminalResult{})
		}},
		{ID: "20260711_subscription_core", Run: func(tx *gorm.DB) error {
			return migrateSubscriptionCore(tx)
		}},
		{ID: "20260711_subscription_order_fulfillment", Run: func(tx *gorm.DB) error {
			if err := migrateSubscriptionOrder(tx); err != nil {
				return err
			}
			return tx.Model(&commerceschema.SubscriptionOrder{}).
				Where("status = ? AND (fulfillment_status = '' OR fulfillment_status IS NULL)", "success").
				Update("fulfillment_status", commerceschema.SubscriptionOrderFulfillmentCompleted).Error
		}},
		{ID: "20260711_gateway_execution_core", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&gatewayschema.RequestExecution{},
				&gatewayschema.GatewayRoutePlan{},
				&gatewayschema.ExecutionAttempt{},
				&gatewayschema.UsageEvidence{},
			)
		}},
		{ID: "20260711_gateway_execution_trace", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&gatewayschema.RequestExecution{},
				&gatewayschema.GatewayRoutePlan{},
				&gatewayschema.ExecutionAttempt{},
				&gatewayschema.UsageEvidence{},
			)
		}},
		{ID: "20260714_user_external_id", Run: migrateUserExternalIDs},
		{ID: "20260724_gateway_route_pools", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&gatewayschema.RoutePool{}, &gatewayschema.RoutePoolMember{})
		}},
		{ID: "20260724_gateway_route_pool_auto_discovery", Run: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&gatewayschema.RoutePool{}, &gatewayschema.RoutePoolMember{})
		}},
	}
	for _, step := range steps {
		var applied schemaMigration
		err := db.Where("id = ?", step.ID).First(&applied).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if dryRun {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := step.Run(tx); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{ID: step.ID}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", step.ID, err)
		}
	}
	return nil
}

func migrateUserExternalIDs(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&identityschema.User{}) {
		return nil
	}
	if !tx.Migrator().HasColumn(&identityschema.User{}, "ExternalId") {
		if err := tx.Migrator().AddColumn(&identityschema.User{}, "ExternalId"); err != nil {
			return err
		}
	}

	var users []identityschema.User
	if err := tx.Unscoped().Where("external_id IS NULL OR external_id = ''").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		var externalID string
		for attempt := 0; attempt < 5; attempt++ {
			generatedID, err := identityschema.GenerateExternalUserID()
			if err != nil {
				return err
			}
			var existing int64
			if err := tx.Unscoped().Model(&identityschema.User{}).Where("external_id = ?", generatedID).Count(&existing).Error; err != nil {
				return err
			}
			if existing == 0 {
				externalID = generatedID
				break
			}
		}
		if externalID == "" {
			return fmt.Errorf("could not allocate a unique external user id")
		}
		if err := tx.Unscoped().Model(&identityschema.User{}).Where("id = ?", user.Id).Update("external_id", externalID).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasIndex(&identityschema.User{}, "idx_users_external_id") {
		return nil
	}
	return tx.Migrator().CreateIndex(&identityschema.User{}, "idx_users_external_id")
}

func migrateSubscriptionCore(tx *gorm.DB) error {
	if !platformdb.UsingSQLite {
		return tx.AutoMigrate(
			&commerceschema.SubscriptionPlan{},
			&commerceschema.SubscriptionOrder{},
			&commerceschema.UserSubscription{},
			&commerceschema.SubscriptionPreConsumeRecord{},
		)
	}
	if err := migrateSubscriptionPlan(tx); err != nil {
		return err
	}
	if err := migrateSubscriptionOrder(tx); err != nil {
		return err
	}
	if err := migrateUserSubscription(tx); err != nil {
		return err
	}
	return tx.AutoMigrate(&commerceschema.SubscriptionPreConsumeRecord{})
}

func migrateSubscriptionPlan(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&commerceschema.SubscriptionPlan{}) {
		return tx.AutoMigrate(&commerceschema.SubscriptionPlan{})
	}
	return ensureSubscriptionPlanTableSQLiteTx(tx)
}

func migrateSubscriptionOrder(tx *gorm.DB) error {
	if !platformdb.UsingSQLite || !tx.Migrator().HasTable(&commerceschema.SubscriptionOrder{}) {
		return tx.AutoMigrate(&commerceschema.SubscriptionOrder{})
	}
	return ensureSubscriptionOrderTableSQLiteTx(tx)
}

func migrateUserSubscription(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&commerceschema.UserSubscription{}) {
		return tx.AutoMigrate(&commerceschema.UserSubscription{})
	}
	return ensureUserSubscriptionTableSQLiteTx(tx)
}
