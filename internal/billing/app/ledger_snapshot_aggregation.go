package app

import (
	"fmt"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	"gorm.io/gorm"
)

type ledgerEntrySnapshotAggregate struct {
	AvailableBalance int64 `gorm:"column:available_balance"`
	GrantedTotal     int64 `gorm:"column:granted_total"`
}

type ledgerSettlementSnapshotAggregate struct {
	ConsumedTotal int64 `gorm:"column:consumed_total"`
	RefundedTotal int64 `gorm:"column:refunded_total"`
}

func aggregateExpectedBalanceSnapshot(db *gorm.DB, accountID string) (billingschema.BillingBalanceSnapshot, error) {
	if db == nil {
		return billingschema.BillingBalanceSnapshot{}, fmt.Errorf("database is required")
	}
	expected := billingschema.BillingBalanceSnapshot{AccountID: accountID}
	var entries ledgerEntrySnapshotAggregate
	if err := db.Model(&billingschema.BillingLedgerEntry{}).
		Where("account_id = ?", accountID).
		Select(`
			COALESCE(SUM(CASE
				WHEN entry_type IN ('grant_credit', 'reserve_release', 'settle_credit') THEN amount
				WHEN entry_type IN ('reserve_hold', 'settle_debit') THEN -amount
				ELSE 0
			END), 0) AS available_balance,
			COALESCE(SUM(CASE WHEN entry_type = 'grant_credit' THEN amount ELSE 0 END), 0) AS granted_total
		`).
		Scan(&entries).Error; err != nil {
		return expected, err
	}
	expected.AvailableBalance = entries.AvailableBalance
	expected.GrantedTotal = entries.GrantedTotal

	settlementTable := billingschema.BillingSettlement{}.TableName()
	reservationTable := billingschema.BillingReservation{}.TableName()
	var settlements ledgerSettlementSnapshotAggregate
	if err := db.Model(&billingschema.BillingSettlement{}).
		Select(`
			COALESCE(SUM(`+settlementTable+`.actual_amount), 0) AS consumed_total,
			COALESCE(SUM(CASE WHEN `+settlementTable+`.delta_amount < 0 THEN -`+settlementTable+`.delta_amount ELSE 0 END), 0) AS refunded_total
		`).
		Joins("JOIN "+reservationTable+" ON "+reservationTable+".reservation_id = "+settlementTable+".reservation_id").
		Where(reservationTable+".account_id = ? AND "+settlementTable+".status = ?", accountID, billingschema.BillingSettlementStatusCompleted).
		Scan(&settlements).Error; err != nil {
		return expected, err
	}
	expected.ConsumedTotal = settlements.ConsumedTotal
	expected.RefundedTotal = settlements.RefundedTotal

	if err := db.Model(&billingschema.BillingReservation{}).
		Where("account_id = ? AND status = ?", accountID, billingschema.BillingReservationStatusOpen).
		Select("COALESCE(SUM(reserved_amount), 0)").
		Scan(&expected.ReservedBalance).Error; err != nil {
		return expected, err
	}
	return expected, nil
}
