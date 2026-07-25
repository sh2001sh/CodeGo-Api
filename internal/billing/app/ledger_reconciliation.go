package app

import (
	"context"
	"fmt"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

type LedgerReconciliation struct {
	AccountID  string                               `json:"account_id"`
	Actual     billingschema.BillingBalanceSnapshot `json:"actual"`
	Expected   billingschema.BillingBalanceSnapshot `json:"expected"`
	Consistent bool                                 `json:"consistent"`
}

func ListLedgerReconciliations(ctx context.Context, limit int) ([]LedgerReconciliation, error) {
	if platformdb.DB == nil {
		return nil, fmt.Errorf("primary database is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var accounts []billingschema.BillingAccount
	if err := platformdb.DB.WithContext(ctx).Order("updated_at desc").Limit(limit).Find(&accounts).Error; err != nil {
		return nil, err
	}
	results := make([]LedgerReconciliation, 0, len(accounts))
	for _, account := range accounts {
		actual, expected, err := ledgerReconciliationForAccount(ctx, account.AccountID)
		if err != nil {
			return nil, err
		}
		results = append(results, LedgerReconciliation{AccountID: account.AccountID, Actual: actual, Expected: expected, Consistent: !snapshotDiffers(actual, expected)})
	}
	return results, nil
}

// CountLedgerInconsistencies checks every billing account without repairing it.
// It is intended for cutover gates and must remain read-only.
func CountLedgerInconsistencies(ctx context.Context) (int, error) {
	if platformdb.DB == nil {
		return 0, fmt.Errorf("primary database is not initialized")
	}
	count := 0
	var accounts []billingschema.BillingAccount
	err := platformdb.DB.WithContext(ctx).Order("account_id asc").FindInBatches(&accounts, 200, func(tx *gorm.DB, _ int) error {
		for _, account := range accounts {
			actual, expected, err := ledgerReconciliationForAccount(ctx, account.AccountID)
			if err != nil {
				return err
			}
			if snapshotDiffers(actual, expected) {
				count++
			}
		}
		return nil
	}).Error
	return count, err
}

func RepairLedgerSnapshot(ctx context.Context, accountID string) error {
	if accountID == "" {
		return fmt.Errorf("account id is required")
	}
	return platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return rebuildBalanceSnapshotTx(tx, accountID)
	})
}

func ledgerReconciliationForAccount(ctx context.Context, accountID string) (billingschema.BillingBalanceSnapshot, billingschema.BillingBalanceSnapshot, error) {
	var actual billingschema.BillingBalanceSnapshot
	if err := platformdb.DB.WithContext(ctx).Where("account_id = ?", accountID).First(&actual).Error; err != nil {
		return actual, billingschema.BillingBalanceSnapshot{}, err
	}
	expected, err := aggregateExpectedBalanceSnapshot(platformdb.DB.WithContext(ctx), accountID)
	if err != nil {
		return actual, expected, err
	}
	return actual, expected, nil
}
