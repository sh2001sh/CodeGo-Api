package app

import (
	"fmt"

	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func restoreSubscriptionLedgerBalanceAfterResetTx(tx *gorm.DB, sub *commerceschema.UserSubscription, operationKey string) error {
	if tx == nil || sub == nil || sub.Id <= 0 || sub.AmountTotal <= 0 {
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
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("account_id = ?", account.AccountID).First(&snapshot).Error; err != nil {
		return err
	}
	targetAvailable := sub.AmountTotal - sub.AmountUsed - snapshot.ReservedBalance
	if targetAvailable <= snapshot.AvailableBalance {
		return nil
	}

	_, err = billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         targetAvailable - snapshot.AvailableBalance,
		IdempotencyKey: fmt.Sprintf("subscription-reset-balance:%d:%s", sub.Id, operationKey),
		ReasonCode:     "subscription_quota_reset",
		ReasonDetail:   "subscription quota reset restored available balance",
		ReferenceType:  "user_subscription",
		ReferenceID:    fmt.Sprintf("%d", sub.Id),
		OperatorType:   "subscription_reset",
		OperatorID:     operationKey,
	})
	return err
}
