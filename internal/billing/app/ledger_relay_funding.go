package app

import (
	"errors"
	"fmt"
	"strings"

	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

// LedgerRelayFunding owns one request-scoped reservation for a wallet-backed relay.
// Legacy quota columns are synchronized as compatibility projections and are never
// used as the source of truth to settle or refund a relay request.
type LedgerRelayFunding struct {
	userID      int
	requestID   string
	accountType string
	accountID   string
	source      string

	reservationID string
	settlementID  string
	reserved      int
	legacyHeld    int
}

func NewLedgerRelayFunding(userID int, requestID string, source string) (*LedgerRelayFunding, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	if strings.TrimSpace(requestID) == "" {
		return nil, fmt.Errorf("request id is required for ledger billing")
	}

	funding := &LedgerRelayFunding{userID: userID, requestID: requestID, source: source}
	switch source {
	case BillingSourceWallet:
		funding.accountType = billingAccountTypeWallet
	default:
		return nil, fmt.Errorf("unsupported ledger funding source: %s", source)
	}
	return funding, nil
}

func (f *LedgerRelayFunding) Source() string {
	return f.source
}

func (f *LedgerRelayFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if f.reservationID != "" {
		if f.reserved != amount {
			return billingdomain.ErrLedgerConflict
		}
		return nil
	}

	account, err := f.ensureAccount()
	if err != nil {
		return err
	}
	if existing, found, err := f.findReservation(); err != nil {
		return err
	} else if found {
		f.accountID = existing.AccountID
		f.reservationID = existing.ReservationID
		f.reserved = int(existing.ReservedAmount)
		if existing.Status == billingschema.BillingReservationStatusOpen {
			f.legacyHeld = f.reserved
		}
		return nil
	}
	reservation, err := billingdomain.CreateReservation(billingdomain.CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      f.requestID,
		ReservedAmount: int64(amount),
		IdempotencyKey: f.idempotencyKey("reserve"),
	})
	if err != nil {
		return err
	}
	if err := f.projectLegacyDelta(amount); err != nil {
		_, releaseErr := billingdomain.ReleaseReservation(billingdomain.ReleaseReservationParams{
			ReservationID:  reservation.ReservationID,
			IdempotencyKey: f.idempotencyKey("release-after-projection-failure"),
			ReasonCode:     "legacy_projection_failed",
		})
		if releaseErr != nil {
			return fmt.Errorf("project legacy balance: %w; release reservation: %v", err, releaseErr)
		}
		return err
	}
	f.accountID = account.AccountID
	f.reservationID = reservation.ReservationID
	f.reserved = amount
	f.legacyHeld = amount
	return nil
}

func (f *LedgerRelayFunding) Settle(delta int) error {
	if f.reservationID == "" {
		return fmt.Errorf("ledger reservation is missing")
	}
	actualAmount := f.reserved + delta
	if actualAmount < 0 {
		actualAmount = 0
	}
	if existing, found, err := f.findSettlement(); err != nil {
		return err
	} else if found {
		f.settlementID = existing.SettlementID
		return nil
	}
	if delta != 0 {
		if err := f.projectLegacyDelta(delta); err != nil {
			return err
		}
		f.legacyHeld += delta
	}
	settlement, err := billingdomain.SettleReservation(billingdomain.SettleReservationParams{
		ReservationID:   f.reservationID,
		UsageEvidenceID: f.requestID,
		ActualAmount:    int64(actualAmount),
		IdempotencyKey:  f.idempotencyKey("settle"),
	})
	if err == nil {
		f.settlementID = settlement.SettlementID
		return nil
	}
	if delta != 0 {
		if rollbackErr := f.projectLegacyDelta(-delta); rollbackErr != nil {
			return fmt.Errorf("settle reservation: %w; restore legacy projection: %v", err, rollbackErr)
		}
		f.legacyHeld -= delta
	}
	return err
}

func (f *LedgerRelayFunding) Refund() error {
	if f.reservationID == "" {
		return nil
	}
	if f.legacyHeld > 0 {
		if err := f.projectLegacyDelta(-f.legacyHeld); err != nil {
			return err
		}
	}
	_, err := billingdomain.ReleaseReservation(billingdomain.ReleaseReservationParams{
		ReservationID:  f.reservationID,
		IdempotencyKey: f.idempotencyKey("release"),
		ReasonCode:     "relay_failed_before_settlement",
	})
	if err == nil {
		f.legacyHeld = 0
		return nil
	}
	if f.legacyHeld > 0 {
		if rollbackErr := f.projectLegacyDelta(f.legacyHeld); rollbackErr != nil {
			return fmt.Errorf("release reservation: %w; restore legacy projection: %v", err, rollbackErr)
		}
	}
	return err
}

func (f *LedgerRelayFunding) ReservationID() string {
	return f.reservationID
}

func (f *LedgerRelayFunding) AccountID() string {
	return f.accountID
}

func (f *LedgerRelayFunding) SettlementID() string {
	return f.settlementID
}

func (f *LedgerRelayFunding) ensureAccount() (*billingschema.BillingAccount, error) {
	if f.accountID != "" {
		return &billingschema.BillingAccount{AccountID: f.accountID}, nil
	}

	legacyBalance, err := f.legacyBalance()
	if err != nil {
		return nil, err
	}
	return ensureMirroredUserAccount(f.userID, f.accountType, legacyBalance)
}

func (f *LedgerRelayFunding) legacyBalance() (int, error) {
	return GetUserWalletQuota(f.userID)
}

func (f *LedgerRelayFunding) idempotencyKey(operation string) string {
	return "relay:" + f.source + ":" + f.requestID + ":" + operation
}

func (f *LedgerRelayFunding) findReservation() (*billingschema.BillingReservation, bool, error) {
	var reservation billingschema.BillingReservation
	err := platformdb.DB.Where("idempotency_key = ?", f.idempotencyKey("reserve")).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &reservation, true, nil
}

func (f *LedgerRelayFunding) findSettlement() (*billingschema.BillingSettlement, bool, error) {
	var settlement billingschema.BillingSettlement
	err := platformdb.DB.Where("idempotency_key = ?", f.idempotencyKey("settle")).First(&settlement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &settlement, true, nil
}

func (f *LedgerRelayFunding) projectLegacyDelta(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return identitystore.DecreaseUserQuota(f.userID, delta)
	}
	return identitystore.IncreaseUserQuota(f.userID, -delta)
}
