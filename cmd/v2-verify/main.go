package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	"gorm.io/gorm"
)

type verificationReport struct {
	MissingMigrations        []string
	MissingWalletAccounts    int
	MissingTokenAccounts     int
	MissingSubscriptionFunds int
	InconsistentLedgers      int
	PendingOutboxEvents      int64
}

func main() {
	strict := flag.Bool("strict", false, "fail when pending outbox events remain")
	flag.Parse()

	platformconfig.IsMasterNode = true
	if path := os.Getenv("SQLITE_PATH"); path != "" {
		platformdb.SQLitePath = path
	}
	if err := platformstore.InitPrimaryDB(); err != nil {
		panic(fmt.Errorf("initialize primary database: %w", err))
	}
	defer platformstore.CloseDatabases()

	report, err := verify(context.Background())
	if err != nil {
		panic(err)
	}
	printReport(report)
	if report.hasFailures(*strict) {
		os.Exit(1)
	}
}

func verify(ctx context.Context) (verificationReport, error) {
	report := verificationReport{}
	if platformdb.DB == nil {
		return report, fmt.Errorf("primary database is not initialized")
	}
	if !platformdb.DB.Migrator().HasTable(migrationTableName()) {
		return report, fmt.Errorf("v2 migration table is missing")
	}
	for _, id := range platformstore.V2MigrationIDs() {
		var count int64
		if err := platformdb.DB.WithContext(ctx).Table(migrationTableName()).Where("id = ?", id).Count(&count).Error; err != nil {
			return report, err
		}
		if count == 0 {
			report.MissingMigrations = append(report.MissingMigrations, id)
		}
	}

	var err error
	if report.MissingWalletAccounts, err = countMissingUserAccounts(ctx, "wallet"); err != nil {
		return report, err
	}
	if report.MissingTokenAccounts, err = countMissingTokenAccounts(ctx); err != nil {
		return report, err
	}
	if report.MissingSubscriptionFunds, err = countMissingSubscriptionAccounts(ctx); err != nil {
		return report, err
	}
	if report.InconsistentLedgers, err = billingapp.CountLedgerInconsistencies(ctx); err != nil {
		return report, err
	}
	if err := platformdb.DB.WithContext(ctx).Model(&billingschema.BillingOutboxEvent{}).
		Where("status = ?", billingschema.BillingOutboxStatusPending).Count(&report.PendingOutboxEvents).Error; err != nil {
		return report, err
	}
	return report, nil
}

func migrationTableName() string {
	if platformdb.UsingPostgreSQL {
		return "platform.schema_migrations"
	}
	return "platform_schema_migrations"
}

func countMissingUserAccounts(ctx context.Context, accountType string) (int, error) {
	return countMissingAccounts(ctx, &identityschema.User{}, func(record *identityschema.User) billingOwner {
		return billingOwner{OwnerType: "user", OwnerID: int64(record.Id), AccountType: accountType}
	})
}

func countMissingTokenAccounts(ctx context.Context) (int, error) {
	return countMissingAccounts(ctx, &identityschema.Token{}, func(record *identityschema.Token) billingOwner {
		if record.UnlimitedQuota {
			return billingOwner{}
		}
		return billingOwner{OwnerType: "token", OwnerID: int64(record.Id), AccountType: "token"}
	})
}

func countMissingSubscriptionAccounts(ctx context.Context) (int, error) {
	return countMissingAccounts(ctx, &commerceschema.UserSubscription{}, func(record *commerceschema.UserSubscription) billingOwner {
		return billingOwner{OwnerType: "user_subscription", OwnerID: int64(record.Id), AccountType: "subscription"}
	})
}

type billingOwner struct {
	OwnerType   string
	OwnerID     int64
	AccountType string
}

func countMissingAccounts[T any](ctx context.Context, model *T, ownerFor func(*T) billingOwner) (int, error) {
	if !platformdb.DB.Migrator().HasTable(model) {
		return 0, nil
	}
	missing := 0
	var records []T
	err := platformdb.DB.WithContext(ctx).Order("id asc").FindInBatches(&records, 500, func(tx *gorm.DB, _ int) error {
		for index := range records {
			owner := ownerFor(&records[index])
			if owner.OwnerType == "" {
				continue
			}
			var count int64
			if err := tx.Model(&billingschema.BillingAccount{}).
				Where("owner_type = ? AND owner_id = ? AND account_type = ? AND quota_unit = ?", owner.OwnerType, owner.OwnerID, owner.AccountType, "quota").
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				missing++
			}
		}
		return nil
	}).Error
	return missing, err
}

func (report verificationReport) hasFailures(strict bool) bool {
	return len(report.MissingMigrations) > 0 ||
		report.MissingWalletAccounts > 0 ||
		report.MissingTokenAccounts > 0 ||
		report.MissingSubscriptionFunds > 0 ||
		report.InconsistentLedgers > 0 ||
		(strict && report.PendingOutboxEvents > 0)
}

func printReport(report verificationReport) {
	fmt.Printf("missing migrations: %d\n", len(report.MissingMigrations))
	for _, id := range report.MissingMigrations {
		fmt.Printf("  %s\n", id)
	}
	fmt.Printf("missing wallet accounts: %d\n", report.MissingWalletAccounts)
	fmt.Printf("missing token accounts: %d\n", report.MissingTokenAccounts)
	fmt.Printf("missing subscription accounts: %d\n", report.MissingSubscriptionFunds)
	fmt.Printf("inconsistent ledger snapshots: %d\n", report.InconsistentLedgers)
	fmt.Printf("pending ledger outbox events: %d\n", report.PendingOutboxEvents)
}
