package app

import (
	"testing"

	"github.com/glebarez/sqlite"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAggregateExpectedBalanceSnapshotIncludesOnlyCompletedSettlements(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
	))

	accountID := "aggregate-account"
	entries := []billingschema.BillingLedgerEntry{
		{AccountID: accountID, EntryType: "grant_credit", Amount: 1_000, IdempotencyKey: "aggregate-grant"},
		{AccountID: accountID, EntryType: "reserve_hold", Amount: 300, IdempotencyKey: "aggregate-hold"},
		{AccountID: accountID, EntryType: "settle_credit", Amount: 50, IdempotencyKey: "aggregate-refund"},
		{AccountID: accountID, EntryType: "adjustment", Amount: 0, IdempotencyKey: "aggregate-adjustment"},
	}
	for _, entry := range entries {
		require.NoError(t, db.Create(&entry).Error)
	}

	completedReservation := billingschema.BillingReservation{ReservationID: "aggregate-completed", AccountID: accountID, Status: billingschema.BillingReservationStatusSettled, IdempotencyKey: "aggregate-completed-reservation", ReservedAmount: 300}
	pendingReservation := billingschema.BillingReservation{ReservationID: "aggregate-pending", AccountID: accountID, Status: billingschema.BillingReservationStatusSettled, IdempotencyKey: "aggregate-pending-reservation", ReservedAmount: 10}
	openReservation := billingschema.BillingReservation{ReservationID: "aggregate-open", AccountID: accountID, Status: billingschema.BillingReservationStatusOpen, IdempotencyKey: "aggregate-open-reservation", ReservedAmount: 80}
	require.NoError(t, db.Create(&completedReservation).Error)
	require.NoError(t, db.Create(&pendingReservation).Error)
	require.NoError(t, db.Create(&openReservation).Error)
	require.NoError(t, db.Create(&billingschema.BillingSettlement{ReservationID: completedReservation.ReservationID, ActualAmount: 250, DeltaAmount: -50, Status: billingschema.BillingSettlementStatusCompleted, IdempotencyKey: "aggregate-completed-settlement"}).Error)
	require.NoError(t, db.Create(&billingschema.BillingSettlement{ReservationID: pendingReservation.ReservationID, ActualAmount: 999, DeltaAmount: -10, Status: billingschema.BillingSettlementStatusPending, IdempotencyKey: "aggregate-pending-settlement"}).Error)

	snapshot, err := aggregateExpectedBalanceSnapshot(db, accountID)
	require.NoError(t, err)
	require.EqualValues(t, 750, snapshot.AvailableBalance)
	require.EqualValues(t, 1_000, snapshot.GrantedTotal)
	require.EqualValues(t, 250, snapshot.ConsumedTotal)
	require.EqualValues(t, 50, snapshot.RefundedTotal)
	require.EqualValues(t, 80, snapshot.ReservedBalance)
}
