package app

import (
	"context"
	"fmt"
	"time"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ledgerWorkerBatchSize = 100
	ledgerWorkerInterval  = 15 * time.Second
)

// StartLedgerWorker begins asynchronous outbox processing for the ledger runtime.
func StartLedgerWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(ledgerWorkerInterval)
		defer ticker.Stop()
		for {
			if _, err := RunLedgerWorkerBatch(ctx, ledgerWorkerBatchSize); err != nil {
				platformobservability.SysError("ledger worker batch failed: " + err.Error())
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// RunLedgerWorkerBatch publishes pending ledger events and rebuilds affected snapshots.
func RunLedgerWorkerBatch(ctx context.Context, limit int) (int, error) {
	if platformdb.DB == nil {
		return 0, fmt.Errorf("primary database is not initialized")
	}
	if limit <= 0 {
		limit = ledgerWorkerBatchSize
	}

	var events []billingschema.BillingOutboxEvent
	if err := platformdb.DB.WithContext(ctx).
		Where("status = ?", billingschema.BillingOutboxStatusPending).
		Order("created_at asc, event_id asc").
		Limit(limit).
		Find(&events).Error; err != nil {
		return 0, err
	}

	processed := 0
	for _, event := range events {
		if err := processLedgerOutboxEvent(ctx, event.EventID); err != nil {
			if markErr := markLedgerOutboxFailure(ctx, event.EventID, err); markErr != nil {
				return processed, fmt.Errorf("process ledger event: %w; mark failure: %v", err, markErr)
			}
			continue
		}
		processed++
	}
	return processed, nil
}

// RebuildBalanceSnapshot recalculates one account projection from immutable ledger data.
func RebuildBalanceSnapshot(ctx context.Context, accountID string) error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	if accountID == "" {
		return fmt.Errorf("account id is required")
	}
	return platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return rebuildBalanceSnapshotTx(tx, accountID)
	})
}

func processLedgerOutboxEvent(ctx context.Context, eventID string) error {
	return platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event billingschema.BillingOutboxEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("event_id = ?", eventID).
			First(&event).Error; err != nil {
			return err
		}
		if event.Status == billingschema.BillingOutboxStatusPublished {
			return nil
		}
		if err := rebuildBalanceSnapshotTx(tx, event.AccountID); err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Model(&event).Updates(map[string]any{
			"status":       billingschema.BillingOutboxStatusPublished,
			"published_at": &now,
			"last_error":   "",
		}).Error
	})
}

func markLedgerOutboxFailure(ctx context.Context, eventID string, cause error) error {
	return platformdb.DB.WithContext(ctx).Model(&billingschema.BillingOutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]any{
			"attempts":   gorm.Expr("attempts + ?", 1),
			"last_error": cause.Error(),
		}).Error
}

func rebuildBalanceSnapshotTx(tx *gorm.DB, accountID string) error {
	var account billingschema.BillingAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ?", accountID).
		First(&account).Error; err != nil {
		return err
	}

	var existing billingschema.BillingBalanceSnapshot
	if err := tx.Where("account_id = ?", accountID).First(&existing).Error; err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	snapshot, err := aggregateExpectedBalanceSnapshot(tx, accountID)
	if err != nil {
		return err
	}
	if snapshotDiffers(existing, snapshot) {
		platformobservability.SysError(fmt.Sprintf(
			"ledger snapshot reconciliation mismatch account_id=%s actual=%+v expected=%+v",
			accountID, existing, snapshot,
		))
	}

	return tx.Save(&snapshot).Error
}

func snapshotDiffers(actual billingschema.BillingBalanceSnapshot, expected billingschema.BillingBalanceSnapshot) bool {
	return actual.AvailableBalance != expected.AvailableBalance ||
		actual.ReservedBalance != expected.ReservedBalance ||
		actual.ConsumedTotal != expected.ConsumedTotal ||
		actual.RefundedTotal != expected.RefundedTotal ||
		actual.GrantedTotal != expected.GrantedTotal
}
