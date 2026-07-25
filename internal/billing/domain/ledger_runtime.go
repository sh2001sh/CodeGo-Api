package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LedgerEntryTypeReserveHold    = "reserve_hold"
	LedgerEntryTypeReserveRelease = "reserve_release"
	LedgerEntryTypeSettleDebit    = "settle_debit"
	LedgerEntryTypeSettleCredit   = "settle_credit"
	LedgerEntryTypeGrantCredit    = "grant_credit"
	LedgerEntryTypeAdjustment     = "adjustment"
)

var (
	ErrLedgerConflict         = errors.New("billing ledger idempotency conflict")
	ErrInsufficientBalance    = errors.New("billing account available balance is insufficient")
	ErrReservationNotOpen     = errors.New("billing reservation is not open")
	ErrReservationNotFound    = errors.New("billing reservation not found")
	ErrSettlementAlreadyExist = errors.New("billing settlement already exists for reservation")
)

type EnsureAccountParams struct {
	AccountType string
	OwnerType   string
	OwnerID     int64
	QuotaUnit   string
}

type CreditAccountParams struct {
	AccountID      string
	Amount         int64
	IdempotencyKey string
	ReasonCode     string
	ReasonDetail   string
	ReferenceType  string
	ReferenceID    string
	OperatorType   string
	OperatorID     string
}

type CreateReservationParams struct {
	AccountID      string
	RequestID      string
	WorkflowID     string
	ReservedAmount int64
	IdempotencyKey string
	ExpiresAt      *time.Time
}

type SettleReservationParams struct {
	ReservationID   string
	UsageEvidenceID string
	ActualAmount    int64
	IdempotencyKey  string
}

type ReleaseReservationParams struct {
	ReservationID  string
	IdempotencyKey string
	ReasonCode     string
}

type RecordAdjustmentParams struct {
	AccountID      string
	IdempotencyKey string
	ReasonCode     string
	ReasonDetail   string
	ReferenceType  string
	ReferenceID    string
	OperatorType   string
	OperatorID     string
	Metadata       json.RawMessage
}

// RecordAdjustment records a zero-balance audit adjustment for state changes such
// as a subscription-period reset. It never changes available or reserved funds.
func RecordAdjustment(params RecordAdjustmentParams) (*billingschema.BillingLedgerEntry, error) {
	if strings.TrimSpace(params.AccountID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id and idempotency_key are required")
	}
	var entry billingschema.BillingLedgerEntry
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		entry, innerErr = recordAdjustmentTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// RecordAdjustmentTx records a zero-balance audit adjustment within an existing transaction.
func RecordAdjustmentTx(tx *gorm.DB, params RecordAdjustmentParams) (*billingschema.BillingLedgerEntry, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(params.AccountID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id and idempotency_key are required")
	}
	entry, err := recordAdjustmentTx(tx, params)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func recordAdjustmentTx(tx *gorm.DB, params RecordAdjustmentParams) (billingschema.BillingLedgerEntry, error) {
	if existing, found, err := findLedgerEntryByIdempotency(tx, params.IdempotencyKey); err != nil {
		return billingschema.BillingLedgerEntry{}, err
	} else if found {
		if existing.AccountID != params.AccountID || existing.EntryType != LedgerEntryTypeAdjustment {
			return billingschema.BillingLedgerEntry{}, ErrLedgerConflict
		}
		return *existing, nil
	}
	entry := billingschema.BillingLedgerEntry{
		AccountID: params.AccountID, ReferenceType: defaultIfEmpty(params.ReferenceType, "adjustment"),
		ReferenceID: defaultIfEmpty(params.ReferenceID, params.AccountID), EntryType: LedgerEntryTypeAdjustment,
		Direction: billingschema.BillingDirectionCredit, Amount: 0, IdempotencyKey: params.IdempotencyKey,
		ReasonCode: defaultIfEmpty(params.ReasonCode, "adjustment"), ReasonDetail: strings.TrimSpace(params.ReasonDetail),
		OperatorType: defaultIfEmpty(params.OperatorType, "system"), OperatorID: params.OperatorID, Metadata: params.Metadata,
	}
	if err := tx.Create(&entry).Error; err != nil {
		return billingschema.BillingLedgerEntry{}, err
	}
	if err := RecordOutboxEvent(tx, OutboxEventInput{AccountID: entry.AccountID, AggregateType: "ledger_entry", AggregateID: entry.EntryID, EventType: "billing.adjustment_recorded", IdempotencyKey: "outbox:" + entry.IdempotencyKey, Payload: entry}); err != nil {
		return billingschema.BillingLedgerEntry{}, err
	}
	return entry, nil
}

// EnsureBillingAccount creates the billing account and its balance snapshot on first use.
func EnsureBillingAccount(params EnsureAccountParams) (*billingschema.BillingAccount, error) {
	if params.OwnerID <= 0 {
		return nil, fmt.Errorf("invalid owner id")
	}
	if strings.TrimSpace(params.OwnerType) == "" || strings.TrimSpace(params.AccountType) == "" {
		return nil, fmt.Errorf("owner_type and account_type are required")
	}
	var account *billingschema.BillingAccount
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		account, innerErr = ensureBillingAccountTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func EnsureBillingAccountTx(tx *gorm.DB, params EnsureAccountParams) (*billingschema.BillingAccount, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if params.OwnerID <= 0 {
		return nil, fmt.Errorf("invalid owner id")
	}
	if strings.TrimSpace(params.OwnerType) == "" || strings.TrimSpace(params.AccountType) == "" {
		return nil, fmt.Errorf("owner_type and account_type are required")
	}
	return ensureBillingAccountTx(tx, params)
}

func ensureBillingAccountTx(tx *gorm.DB, params EnsureAccountParams) (*billingschema.BillingAccount, error) {
	quotaUnit := strings.TrimSpace(params.QuotaUnit)
	if quotaUnit == "" {
		quotaUnit = "quota"
	}

	var account billingschema.BillingAccount
	accountType := strings.TrimSpace(params.AccountType)
	if err := tx.Where("owner_type = ? AND owner_id = ? AND account_type = ? AND quota_unit = ?",
		strings.TrimSpace(params.OwnerType),
		params.OwnerID,
		accountType,
		quotaUnit,
	).
		First(&account).Error; err == nil {
		if err := ensureBalanceSnapshot(tx, account.AccountID); err != nil {
			return nil, err
		}
		return &account, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	account = billingschema.BillingAccount{
		AccountType: accountType,
		OwnerType:   strings.TrimSpace(params.OwnerType),
		OwnerID:     params.OwnerID,
		QuotaUnit:   quotaUnit,
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, err
	}
	if err := ensureBalanceSnapshot(tx, account.AccountID); err != nil {
		return nil, err
	}
	return &account, nil
}

// CreditAccount grants available balance through a ledger entry and updates snapshot totals.
func CreditAccount(params CreditAccountParams) (*billingschema.BillingLedgerEntry, error) {
	if strings.TrimSpace(params.AccountID) == "" || params.Amount <= 0 || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id, amount and idempotency_key are required")
	}

	var entry *billingschema.BillingLedgerEntry
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		entry, innerErr = creditAccountTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func CreditAccountTx(tx *gorm.DB, params CreditAccountParams) (*billingschema.BillingLedgerEntry, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(params.AccountID) == "" || params.Amount <= 0 || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id, amount and idempotency_key are required")
	}
	return creditAccountTx(tx, params)
}

func creditAccountTx(tx *gorm.DB, params CreditAccountParams) (*billingschema.BillingLedgerEntry, error) {
	if existing, found, err := findLedgerEntryByIdempotency(tx, params.IdempotencyKey); err != nil {
		return nil, err
	} else if found {
		if existing.AccountID != params.AccountID || existing.Amount != params.Amount || existing.EntryType != LedgerEntryTypeGrantCredit {
			return nil, ErrLedgerConflict
		}
		return existing, nil
	}

	snapshot, err := ensureAndLockBalanceSnapshot(tx, params.AccountID)
	if err != nil {
		return nil, err
	}
	snapshot.AvailableBalance += params.Amount
	snapshot.GrantedTotal += params.Amount
	if err := tx.Save(snapshot).Error; err != nil {
		return nil, err
	}

	balanceAfter := snapshot.AvailableBalance
	entry := &billingschema.BillingLedgerEntry{
		AccountID:      params.AccountID,
		ReferenceType:  defaultIfEmpty(strings.TrimSpace(params.ReferenceType), "migration"),
		ReferenceID:    defaultIfEmpty(strings.TrimSpace(params.ReferenceID), params.AccountID),
		EntryType:      LedgerEntryTypeGrantCredit,
		Direction:      billingschema.BillingDirectionCredit,
		Amount:         params.Amount,
		BalanceAfter:   &balanceAfter,
		IdempotencyKey: strings.TrimSpace(params.IdempotencyKey),
		ReasonCode:     defaultIfEmpty(strings.TrimSpace(params.ReasonCode), "grant_credit"),
		ReasonDetail:   strings.TrimSpace(params.ReasonDetail),
		OperatorType:   defaultIfEmpty(strings.TrimSpace(params.OperatorType), "system"),
		OperatorID:     strings.TrimSpace(params.OperatorID),
	}
	if err := tx.Create(entry).Error; err != nil {
		return nil, err
	}
	if err := RecordOutboxEvent(tx, OutboxEventInput{
		AccountID:      entry.AccountID,
		AggregateType:  "ledger_entry",
		AggregateID:    entry.EntryID,
		EventType:      "billing.credit_granted",
		IdempotencyKey: "outbox:" + entry.IdempotencyKey,
		Payload:        entry,
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// CreateReservation opens a reservation and holds funds in the balance snapshot.
func CreateReservation(params CreateReservationParams) (*billingschema.BillingReservation, error) {
	if strings.TrimSpace(params.AccountID) == "" || params.ReservedAmount <= 0 || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id, reserved_amount and idempotency_key are required")
	}

	var reservation billingschema.BillingReservation
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		reservation, innerErr = createReservationTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func CreateReservationTx(tx *gorm.DB, params CreateReservationParams) (*billingschema.BillingReservation, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(params.AccountID) == "" || params.ReservedAmount <= 0 || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("account_id, reserved_amount and idempotency_key are required")
	}
	reservation, err := createReservationTx(tx, params)
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func createReservationTx(tx *gorm.DB, params CreateReservationParams) (billingschema.BillingReservation, error) {
	var reservation billingschema.BillingReservation
	if existing, found, err := findReservationByIdempotency(tx, params.IdempotencyKey); err != nil {
		return reservation, err
	} else if found {
		if existing.AccountID != params.AccountID || existing.ReservedAmount != params.ReservedAmount || existing.RequestID != strings.TrimSpace(params.RequestID) || existing.WorkflowID != strings.TrimSpace(params.WorkflowID) {
			return reservation, ErrLedgerConflict
		}
		return *existing, nil
	}

	snapshot, err := ensureAndLockBalanceSnapshot(tx, params.AccountID)
	if err != nil {
		return reservation, err
	}
	if snapshot.AvailableBalance < params.ReservedAmount {
		return reservation, ErrInsufficientBalance
	}

	snapshot.AvailableBalance -= params.ReservedAmount
	snapshot.ReservedBalance += params.ReservedAmount
	if err := tx.Save(snapshot).Error; err != nil {
		return reservation, err
	}

	reservation = billingschema.BillingReservation{
		AccountID:      params.AccountID,
		RequestID:      strings.TrimSpace(params.RequestID),
		WorkflowID:     strings.TrimSpace(params.WorkflowID),
		ReservedAmount: params.ReservedAmount,
		Status:         billingschema.BillingReservationStatusOpen,
		IdempotencyKey: strings.TrimSpace(params.IdempotencyKey),
		ExpiresAt:      params.ExpiresAt,
	}
	if err := tx.Create(&reservation).Error; err != nil {
		return reservation, err
	}

	balanceAfter := snapshot.AvailableBalance
	entry := billingschema.BillingLedgerEntry{
		AccountID:      params.AccountID,
		ReferenceType:  "reservation",
		ReferenceID:    reservation.ReservationID,
		EntryType:      LedgerEntryTypeReserveHold,
		Direction:      billingschema.BillingDirectionDebit,
		Amount:         params.ReservedAmount,
		BalanceAfter:   &balanceAfter,
		IdempotencyKey: "entry:" + reservation.IdempotencyKey,
		ReasonCode:     "reservation_hold",
		OperatorType:   "system",
	}
	if err := tx.Create(&entry).Error; err != nil {
		return reservation, err
	}
	if err := RecordOutboxEvent(tx, OutboxEventInput{
		AccountID:      reservation.AccountID,
		AggregateType:  "reservation",
		AggregateID:    reservation.ReservationID,
		EventType:      "billing.reservation_created",
		IdempotencyKey: "outbox:" + reservation.IdempotencyKey,
		Payload:        reservation,
	}); err != nil {
		return reservation, err
	}
	return reservation, nil
}

// ReleaseReservation releases an open reservation back to available balance.
func ReleaseReservation(params ReleaseReservationParams) (*billingschema.BillingReservation, error) {
	if strings.TrimSpace(params.ReservationID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("reservation_id and idempotency_key are required")
	}

	var reservation billingschema.BillingReservation
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		reservation, innerErr = releaseReservationTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

// ReleaseReservationTx releases an open reservation inside the caller's transaction.
func ReleaseReservationTx(tx *gorm.DB, params ReleaseReservationParams) (*billingschema.BillingReservation, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(params.ReservationID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("reservation_id and idempotency_key are required")
	}
	reservation, err := releaseReservationTx(tx, params)
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func releaseReservationTx(tx *gorm.DB, params ReleaseReservationParams) (billingschema.BillingReservation, error) {
	var reservation billingschema.BillingReservation
	current, err := lockReservation(tx, params.ReservationID)
	if err != nil {
		return reservation, err
	}
	if current.Status == billingschema.BillingReservationStatusReleased {
		return *current, nil
	}
	if current.Status != billingschema.BillingReservationStatusOpen {
		return reservation, ErrReservationNotOpen
	}

	if existing, found, err := findLedgerEntryByIdempotency(tx, params.IdempotencyKey); err != nil {
		return reservation, err
	} else if found {
		if existing.ReferenceID != current.ReservationID || existing.EntryType != LedgerEntryTypeReserveRelease {
			return reservation, ErrLedgerConflict
		}
		current.Status = billingschema.BillingReservationStatusReleased
		if err := tx.Model(current).Update("status", current.Status).Error; err != nil {
			return reservation, err
		}
		return *current, nil
	}

	snapshot, err := ensureAndLockBalanceSnapshot(tx, current.AccountID)
	if err != nil {
		return reservation, err
	}
	if snapshot.ReservedBalance < current.ReservedAmount {
		return reservation, fmt.Errorf("reserved balance underflow")
	}
	snapshot.AvailableBalance += current.ReservedAmount
	snapshot.ReservedBalance -= current.ReservedAmount
	if err := tx.Save(snapshot).Error; err != nil {
		return reservation, err
	}

	current.Status = billingschema.BillingReservationStatusReleased
	if err := tx.Save(current).Error; err != nil {
		return reservation, err
	}

	balanceAfter := snapshot.AvailableBalance
	entry := billingschema.BillingLedgerEntry{
		AccountID:      current.AccountID,
		ReferenceType:  "reservation",
		ReferenceID:    current.ReservationID,
		EntryType:      LedgerEntryTypeReserveRelease,
		Direction:      billingschema.BillingDirectionCredit,
		Amount:         current.ReservedAmount,
		BalanceAfter:   &balanceAfter,
		IdempotencyKey: strings.TrimSpace(params.IdempotencyKey),
		ReasonCode:     defaultIfEmpty(strings.TrimSpace(params.ReasonCode), "reservation_release"),
		OperatorType:   "system",
	}
	if err := tx.Create(&entry).Error; err != nil {
		return reservation, err
	}
	if err := RecordOutboxEvent(tx, OutboxEventInput{
		AccountID:      current.AccountID,
		AggregateType:  "reservation",
		AggregateID:    current.ReservationID,
		EventType:      "billing.reservation_released",
		IdempotencyKey: "outbox:" + entry.IdempotencyKey,
		Payload:        current,
	}); err != nil {
		return reservation, err
	}
	return *current, nil
}

// SettleReservation finalizes an open reservation and applies delta debit/credit.
func SettleReservation(params SettleReservationParams) (*billingschema.BillingSettlement, error) {
	if strings.TrimSpace(params.ReservationID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("reservation_id and idempotency_key are required")
	}
	if params.ActualAmount < 0 {
		return nil, fmt.Errorf("actual_amount cannot be negative")
	}

	var settlement billingschema.BillingSettlement
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		var innerErr error
		settlement, innerErr = settleReservationTx(tx, params)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return &settlement, nil
}

func SettleReservationTx(tx *gorm.DB, params SettleReservationParams) (*billingschema.BillingSettlement, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(params.ReservationID) == "" || strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("reservation_id and idempotency_key are required")
	}
	if params.ActualAmount < 0 {
		return nil, fmt.Errorf("actual_amount cannot be negative")
	}
	settlement, err := settleReservationTx(tx, params)
	if err != nil {
		return nil, err
	}
	return &settlement, nil
}

func settleReservationTx(tx *gorm.DB, params SettleReservationParams) (billingschema.BillingSettlement, error) {
	var settlement billingschema.BillingSettlement
	if existing, found, err := findSettlementByIdempotency(tx, params.IdempotencyKey); err != nil {
		return settlement, err
	} else if found {
		if existing.ReservationID != params.ReservationID || existing.ActualAmount != params.ActualAmount {
			return settlement, ErrLedgerConflict
		}
		return *existing, nil
	}

	current, err := lockReservation(tx, params.ReservationID)
	if err != nil {
		return settlement, err
	}
	if current.Status != billingschema.BillingReservationStatusOpen {
		if current.Status == billingschema.BillingReservationStatusSettled {
			return settlement, ErrSettlementAlreadyExist
		}
		return settlement, ErrReservationNotOpen
	}

	snapshot, err := ensureAndLockBalanceSnapshot(tx, current.AccountID)
	if err != nil {
		return settlement, err
	}
	if snapshot.ReservedBalance < current.ReservedAmount {
		return settlement, fmt.Errorf("reserved balance underflow")
	}

	delta := params.ActualAmount - current.ReservedAmount
	if delta > 0 && snapshot.AvailableBalance < delta {
		return settlement, ErrInsufficientBalance
	}

	snapshot.ReservedBalance -= current.ReservedAmount
	snapshot.ConsumedTotal += params.ActualAmount
	switch {
	case delta > 0:
		snapshot.AvailableBalance -= delta
	case delta < 0:
		snapshot.AvailableBalance += -delta
		snapshot.RefundedTotal += -delta
	}
	if err := tx.Save(snapshot).Error; err != nil {
		return settlement, err
	}

	settlement = billingschema.BillingSettlement{
		ReservationID:   current.ReservationID,
		UsageEvidenceID: strings.TrimSpace(params.UsageEvidenceID),
		ActualAmount:    params.ActualAmount,
		DeltaAmount:     delta,
		Status:          billingschema.BillingSettlementStatusCompleted,
		IdempotencyKey:  strings.TrimSpace(params.IdempotencyKey),
	}
	if err := tx.Create(&settlement).Error; err != nil {
		return settlement, err
	}

	current.Status = billingschema.BillingReservationStatusSettled
	if err := tx.Save(current).Error; err != nil {
		return settlement, err
	}

	if delta != 0 {
		entryType := LedgerEntryTypeSettleDebit
		direction := billingschema.BillingDirectionDebit
		amount := delta
		if delta < 0 {
			entryType = LedgerEntryTypeSettleCredit
			direction = billingschema.BillingDirectionCredit
			amount = -delta
		}
		balanceAfter := snapshot.AvailableBalance
		entry := billingschema.BillingLedgerEntry{
			AccountID:      current.AccountID,
			ReferenceType:  "settlement",
			ReferenceID:    settlement.SettlementID,
			EntryType:      entryType,
			Direction:      direction,
			Amount:         amount,
			BalanceAfter:   &balanceAfter,
			IdempotencyKey: "entry:" + settlement.IdempotencyKey,
			ReasonCode:     "reservation_settlement",
			OperatorType:   "system",
		}
		if err := tx.Create(&entry).Error; err != nil {
			return settlement, err
		}
	}
	if err := RecordOutboxEvent(tx, OutboxEventInput{
		AccountID:      current.AccountID,
		AggregateType:  "settlement",
		AggregateID:    settlement.SettlementID,
		EventType:      "billing.reservation_settled",
		IdempotencyKey: "outbox:" + settlement.IdempotencyKey,
		Payload:        settlement,
	}); err != nil {
		return settlement, err
	}
	return settlement, nil
}

func ensureBalanceSnapshot(tx *gorm.DB, accountID string) error {
	var snapshot billingschema.BillingBalanceSnapshot
	if err := tx.Where("account_id = ?", accountID).First(&snapshot).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := tx.Create(&billingschema.BillingBalanceSnapshot{AccountID: accountID}).Error; err != nil {
		var retry billingschema.BillingBalanceSnapshot
		if readErr := tx.Where("account_id = ?", accountID).First(&retry).Error; readErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func ensureAndLockBalanceSnapshot(tx *gorm.DB, accountID string) (*billingschema.BillingBalanceSnapshot, error) {
	if err := ensureBalanceSnapshot(tx, accountID); err != nil {
		return nil, err
	}
	var snapshot billingschema.BillingBalanceSnapshot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ?", accountID).
		First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func lockReservation(tx *gorm.DB, reservationID string) (*billingschema.BillingReservation, error) {
	var reservation billingschema.BillingReservation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reservation_id = ?", reservationID).
		First(&reservation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReservationNotFound
		}
		return nil, err
	}
	return &reservation, nil
}

func findReservationByIdempotency(tx *gorm.DB, idempotencyKey string) (*billingschema.BillingReservation, bool, error) {
	var reservation billingschema.BillingReservation
	if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&reservation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &reservation, true, nil
}

func findSettlementByIdempotency(tx *gorm.DB, idempotencyKey string) (*billingschema.BillingSettlement, bool, error) {
	var settlement billingschema.BillingSettlement
	if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&settlement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &settlement, true, nil
}

func findLedgerEntryByIdempotency(tx *gorm.DB, idempotencyKey string) (*billingschema.BillingLedgerEntry, bool, error) {
	var entry billingschema.BillingLedgerEntry
	if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &entry, true, nil
}

func defaultIfEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
