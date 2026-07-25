package domain

import (
	"github.com/glebarez/sqlite"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

func TestLedgerReservationIdempotency(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account, err := EnsureBillingAccount(EnsureAccountParams{
		AccountType: "wallet",
		OwnerType:   "user",
		OwnerID:     1001,
	})
	require.NoError(t, err)

	_, err = CreditAccount(CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         1000,
		IdempotencyKey: "credit-1",
		ReferenceType:  "migration",
		ReferenceID:    "seed-1",
	})
	require.NoError(t, err)

	first, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-1",
		ReservedAmount: 300,
		IdempotencyKey: "reserve-1",
	})
	require.NoError(t, err)

	second, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-1",
		ReservedAmount: 300,
		IdempotencyKey: "reserve-1",
	})
	require.NoError(t, err)
	require.Equal(t, first.ReservationID, second.ReservationID)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(700), snapshot.AvailableBalance)
	require.Equal(t, int64(300), snapshot.ReservedBalance)

	var reservationCount int64
	var entryCount int64
	require.NoError(t, platformdb.DB.Model(&billingschema.BillingReservation{}).Count(&reservationCount).Error)
	require.NoError(t, platformdb.DB.Model(&billingschema.BillingLedgerEntry{}).Count(&entryCount).Error)
	require.Equal(t, int64(1), reservationCount)
	require.Equal(t, int64(2), entryCount)
}

func TestLedgerSettlementPositiveDelta(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account := seedAccountWithCredit(t, 1002, 1000)
	reservation, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-2",
		ReservedAmount: 300,
		IdempotencyKey: "reserve-2",
	})
	require.NoError(t, err)

	settlement, err := SettleReservation(SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   450,
		IdempotencyKey: "settle-2",
	})
	require.NoError(t, err)
	require.Equal(t, int64(150), settlement.DeltaAmount)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(550), snapshot.AvailableBalance)
	require.Equal(t, int64(0), snapshot.ReservedBalance)
	require.Equal(t, int64(450), snapshot.ConsumedTotal)
	require.Equal(t, int64(0), snapshot.RefundedTotal)
}

func TestLedgerSettlementNegativeDelta(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account := seedAccountWithCredit(t, 1003, 1000)
	reservation, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-3",
		ReservedAmount: 300,
		IdempotencyKey: "reserve-3",
	})
	require.NoError(t, err)

	settlement, err := SettleReservation(SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   200,
		IdempotencyKey: "settle-3",
	})
	require.NoError(t, err)
	require.Equal(t, int64(-100), settlement.DeltaAmount)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(800), snapshot.AvailableBalance)
	require.Equal(t, int64(0), snapshot.ReservedBalance)
	require.Equal(t, int64(200), snapshot.ConsumedTotal)
	require.Equal(t, int64(100), snapshot.RefundedTotal)
}

func TestLedgerReleaseReservation(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account := seedAccountWithCredit(t, 1004, 1000)
	reservation, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-4",
		ReservedAmount: 250,
		IdempotencyKey: "reserve-4",
	})
	require.NoError(t, err)

	released, err := ReleaseReservation(ReleaseReservationParams{
		ReservationID:  reservation.ReservationID,
		IdempotencyKey: "release-4",
	})
	require.NoError(t, err)
	require.Equal(t, billingschema.BillingReservationStatusReleased, released.Status)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(1000), snapshot.AvailableBalance)
	require.Equal(t, int64(0), snapshot.ReservedBalance)
}

func TestLedgerReservationInsufficientBalance(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account := seedAccountWithCredit(t, 1005, 100)
	_, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-5",
		ReservedAmount: 200,
		IdempotencyKey: "reserve-5",
	})
	require.ErrorIs(t, err, ErrInsufficientBalance)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(100), snapshot.AvailableBalance)
	require.Equal(t, int64(0), snapshot.ReservedBalance)
}

func TestLedgerSettlementIdempotency(t *testing.T) {
	db := openLedgerTestDB(t)
	platformdb.DB = db

	account := seedAccountWithCredit(t, 1006, 1000)
	reservation, err := CreateReservation(CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      "req-6",
		ReservedAmount: 300,
		IdempotencyKey: "reserve-6",
	})
	require.NoError(t, err)

	first, err := SettleReservation(SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   450,
		IdempotencyKey: "settle-6",
	})
	require.NoError(t, err)

	second, err := SettleReservation(SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   450,
		IdempotencyKey: "settle-6",
	})
	require.NoError(t, err)
	require.Equal(t, first.SettlementID, second.SettlementID)

	snapshot := loadSnapshot(t, account.AccountID)
	require.Equal(t, int64(550), snapshot.AvailableBalance)
	require.Equal(t, int64(0), snapshot.ReservedBalance)
	require.Equal(t, int64(450), snapshot.ConsumedTotal)

	var settlementCount int64
	var entryCount int64
	require.NoError(t, platformdb.DB.Model(&billingschema.BillingSettlement{}).Count(&settlementCount).Error)
	require.NoError(t, platformdb.DB.Model(&billingschema.BillingLedgerEntry{}).Count(&entryCount).Error)
	require.Equal(t, int64(1), settlementCount)
	require.Equal(t, int64(3), entryCount)
}

func openLedgerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false
	require.NoError(t, db.AutoMigrate(
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
	))
	return db
}

func seedAccountWithCredit(t *testing.T, ownerID int64, amount int64) *billingschema.BillingAccount {
	t.Helper()
	account, err := EnsureBillingAccount(EnsureAccountParams{
		AccountType: "wallet",
		OwnerType:   "user",
		OwnerID:     ownerID,
	})
	require.NoError(t, err)
	_, err = CreditAccount(CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         amount,
		IdempotencyKey: "credit-seed-" + account.AccountID,
		ReferenceType:  "migration",
		ReferenceID:    account.AccountID,
	})
	require.NoError(t, err)
	return account
}

func loadSnapshot(t *testing.T, accountID string) *billingschema.BillingBalanceSnapshot {
	t.Helper()
	var snapshot billingschema.BillingBalanceSnapshot
	require.NoError(t, platformdb.DB.Where("account_id = ?", accountID).First(&snapshot).Error)
	return &snapshot
}
