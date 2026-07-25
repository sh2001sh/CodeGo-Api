package app

import (
	"fmt"

	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// replenishSubscriptionLedgerForCycleTx makes a renewed or upgraded
// subscription's ledger balance available for its refreshed quota cycle.
func replenishSubscriptionLedgerForCycleTx(tx *gorm.DB, sub *commerceschema.UserSubscription, cycle string) error {
	if tx == nil || sub == nil || sub.Id <= 0 {
		return fmt.Errorf("invalid subscription ledger reconciliation args")
	}

	targetAvailable := sub.AmountTotal - sub.AmountUsed
	if targetAvailable <= 0 {
		return nil
	}

	account, err := billingdomain.EnsureBillingAccountTx(tx, billingdomain.EnsureAccountParams{
		AccountType: "subscription",
		OwnerType:   "user_subscription",
		OwnerID:     int64(sub.Id),
		QuotaUnit:   "quota",
	})
	if err != nil {
		return err
	}

	var snapshot billingschema.BillingBalanceSnapshot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ?", account.AccountID).
		First(&snapshot).Error; err != nil {
		return err
	}
	if snapshot.AvailableBalance >= targetAvailable {
		return nil
	}

	_, err = billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         targetAvailable - snapshot.AvailableBalance,
		IdempotencyKey: fmt.Sprintf("subscription-cycle:%s:%d:%d:%d:%d", cycle, sub.Id, sub.StartTime, sub.AmountTotal, sub.AmountUsed),
		ReasonCode:     "subscription_cycle_" + cycle,
		ReferenceType:  "user_subscription",
		ReferenceID:    fmt.Sprintf("%d", sub.Id),
		OperatorType:   "commerce",
		OperatorID:     cycle,
	})
	return err
}
