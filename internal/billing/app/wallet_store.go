package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
	"strings"
)

const (
	billingOwnerTypeUser           = "user"
	billingQuotaUnitQuota          = "quota"
	billingAccountTypeWallet       = "wallet"
	billingReservationRequestIDMax = 64
)

type mirroredWalletStore struct {
	accountType string
	readBalance func(userID int) (int, error)
	applyDelta  func(userID int, delta int) error
}

type mirroredWalletTxStore struct {
	accountType string
	readBalance func(tx *gorm.DB, userID int) (int, error)
	applyDelta  func(tx *gorm.DB, userID int, amount int) error
}

func GetUserWalletQuota(userID int) (int, error) {
	return getLedgerBackedWalletBalance(userID, billingAccountTypeWallet, func() (int, error) {
		return identitystore.LoadUserQuota(userID, false)
	})
}

// getLedgerBackedWalletBalance treats the ledger snapshot as canonical after the
// account has been bootstrapped. The legacy user column remains a compatibility
// projection for code paths that have not yet been migrated.
func getLedgerBackedWalletBalance(userID int, accountType string, legacyRead func() (int, error)) (int, error) {
	if userID <= 0 {
		return 0, errors.New("invalid user id")
	}
	var account billingschema.BillingAccount
	err := platformdb.DB.Where("owner_type = ? AND owner_id = ? AND account_type = ? AND quota_unit = ?",
		billingOwnerTypeUser, userID, accountType, billingQuotaUnitQuota,
	).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return legacyRead()
	}
	if isMissingBillingSchema(err) {
		return legacyRead()
	}
	if err != nil {
		return 0, err
	}

	var snapshot billingschema.BillingBalanceSnapshot
	if err := platformdb.DB.Where("account_id = ?", account.AccountID).First(&snapshot).Error; err != nil {
		return 0, err
	}
	return int(snapshot.AvailableBalance), nil
}

func isMissingBillingSchema(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") || strings.Contains(message, "does not exist")
}

func AdjustWalletQuota(userID int, delta int) error {
	return adjustUserWalletQuota(userID, delta, mirroredWalletStore{
		accountType: billingAccountTypeWallet,
		readBalance: GetUserWalletQuota,
		applyDelta: func(targetUserID int, targetDelta int) error {
			if targetDelta > 0 {
				return identitystore.DecreaseUserQuota(targetUserID, targetDelta)
			}
			return identitystore.IncreaseUserQuota(targetUserID, -targetDelta)
		},
	})
}

func SetWalletQuota(userID int, targetBalance int) error {
	return setUserWalletQuota(userID, targetBalance, GetUserWalletQuota, AdjustWalletQuota)
}

func CreditWalletQuotaTx(tx *gorm.DB, userID int, amount int, idempotencyKey string, reasonCode string) error {
	return creditUserWalletQuotaTx(tx, userID, amount, idempotencyKey, reasonCode, mirroredWalletTxStore{
		accountType: billingAccountTypeWallet,
		readBalance: getUserWalletQuotaTx,
		applyDelta:  increaseUserWalletQuotaTx,
	})
}

func setUserWalletQuota(userID int, targetBalance int, readBalance func(int) (int, error), applyDelta func(int, int) error) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if targetBalance < 0 {
		return errors.New("target balance cannot be negative")
	}
	currentBalance, err := readBalance(userID)
	if err != nil {
		return err
	}
	return applyDelta(userID, currentBalance-targetBalance)
}

func adjustUserWalletQuota(userID int, delta int, mirrored mirroredWalletStore) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if delta == 0 {
		return nil
	}

	legacyBalance, err := mirrored.readBalance(userID)
	if err != nil {
		return err
	}
	account, err := ensureMirroredUserAccount(userID, mirrored.accountType, legacyBalance)
	if err != nil {
		return err
	}
	if err := reconcileAccountBalance(account, userID, legacyBalance); err != nil {
		return err
	}

	operationID := platformruntime.GetUUID()
	if err := applyLedgerDelta(account, userID, delta, operationID, "ledger_sync_apply"); err != nil {
		return err
	}
	if err := mirrored.applyDelta(userID, delta); err != nil {
		compensationErr := applyLedgerDelta(account, userID, -delta, platformruntime.GetUUID(), "ledger_sync_compensate")
		if compensationErr != nil {
			return errors.Join(err, fmt.Errorf("ledger compensation failed: %w", compensationErr))
		}
		return err
	}
	return nil
}

func creditUserWalletQuotaTx(tx *gorm.DB, userID int, amount int, idempotencyKey string, reasonCode string, mirrored mirroredWalletTxStore) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	if idempotencyKey == "" {
		return errors.New("idempotency key is required")
	}

	legacyBalance, err := mirrored.readBalance(tx, userID)
	if err != nil {
		return err
	}
	account, err := ensureMirroredUserAccountTx(tx, userID, mirrored.accountType, legacyBalance)
	if err != nil {
		return err
	}
	if err := reconcileAccountBalanceTx(tx, account, userID, legacyBalance); err != nil {
		return err
	}
	_, err = billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         int64(amount),
		IdempotencyKey: idempotencyKey,
		ReasonCode:     defaultReasonCode(reasonCode, "ledger_sync_credit"),
		ReferenceType:  "user",
		ReferenceID:    fmt.Sprintf("%d", userID),
		OperatorType:   "ledger_sync",
		OperatorID:     idempotencyKey,
	})
	if err != nil {
		return err
	}
	if err := mirrored.applyDelta(tx, userID, amount); err != nil {
		return err
	}
	return nil
}

func ensureMirroredUserAccount(userID int, accountType string, legacyBalance int) (*billingschema.BillingAccount, error) {
	account, err := billingdomain.EnsureBillingAccount(billingdomain.EnsureAccountParams{
		AccountType: accountType,
		OwnerType:   billingOwnerTypeUser,
		OwnerID:     int64(userID),
		QuotaUnit:   billingQuotaUnitQuota,
	})
	if err != nil {
		return nil, err
	}
	snapshot, err := loadBalanceSnapshot(account.AccountID)
	if err != nil {
		return nil, err
	}
	if hasNonZeroSnapshot(snapshot) || legacyBalance <= 0 {
		return account, nil
	}
	_, err = billingdomain.CreditAccount(billingdomain.CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         int64(legacyBalance),
		IdempotencyKey: fmt.Sprintf("mirror-bootstrap:user:%d:%s", userID, accountType),
		ReasonCode:     "mirror_bootstrap",
		ReferenceType:  "user",
		ReferenceID:    fmt.Sprintf("%d", userID),
		OperatorType:   "ledger_sync",
		OperatorID:     "mirror_bootstrap",
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func ensureMirroredUserAccountTx(tx *gorm.DB, userID int, accountType string, legacyBalance int) (*billingschema.BillingAccount, error) {
	account, err := billingdomain.EnsureBillingAccountTx(tx, billingdomain.EnsureAccountParams{
		AccountType: accountType,
		OwnerType:   billingOwnerTypeUser,
		OwnerID:     int64(userID),
		QuotaUnit:   billingQuotaUnitQuota,
	})
	if err != nil {
		return nil, err
	}
	snapshot, err := loadBalanceSnapshotTx(tx, account.AccountID)
	if err != nil {
		return nil, err
	}
	if hasNonZeroSnapshot(snapshot) || legacyBalance <= 0 {
		return account, nil
	}
	_, err = billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
		AccountID:      account.AccountID,
		Amount:         int64(legacyBalance),
		IdempotencyKey: fmt.Sprintf("mirror-bootstrap:user:%d:%s", userID, accountType),
		ReasonCode:     "mirror_bootstrap",
		ReferenceType:  "user",
		ReferenceID:    fmt.Sprintf("%d", userID),
		OperatorType:   "ledger_sync",
		OperatorID:     "mirror_bootstrap",
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func reconcileAccountBalance(account *billingschema.BillingAccount, userID int, legacyBalance int) error {
	if account == nil {
		return errors.New("billing account is required")
	}
	snapshot, err := loadBalanceSnapshot(account.AccountID)
	if err != nil {
		return err
	}
	diff := legacyBalance - int(snapshot.AvailableBalance)
	if diff == 0 {
		return nil
	}
	return applyLedgerDelta(account, userID, -diff, fmt.Sprintf("reconcile:%s", platformruntime.GetUUID()), "ledger_sync_reconcile")
}

func reconcileAccountBalanceTx(tx *gorm.DB, account *billingschema.BillingAccount, userID int, legacyBalance int) error {
	if account == nil {
		return errors.New("billing account is required")
	}
	snapshot, err := loadBalanceSnapshotTx(tx, account.AccountID)
	if err != nil {
		return err
	}
	diff := legacyBalance - int(snapshot.AvailableBalance)
	if diff == 0 {
		return nil
	}
	return applyLedgerDeltaTx(tx, account, userID, -diff, fmt.Sprintf("reconcile:%s", platformruntime.GetUUID()), "ledger_sync_reconcile")
}

func loadBalanceSnapshot(accountID string) (*billingschema.BillingBalanceSnapshot, error) {
	var snapshot billingschema.BillingBalanceSnapshot
	if err := platformdb.DB.Where("account_id = ?", accountID).First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func loadBalanceSnapshotTx(tx *gorm.DB, accountID string) (*billingschema.BillingBalanceSnapshot, error) {
	var snapshot billingschema.BillingBalanceSnapshot
	if err := tx.Where("account_id = ?", accountID).First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func hasNonZeroSnapshot(snapshot *billingschema.BillingBalanceSnapshot) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.AvailableBalance != 0 ||
		snapshot.ReservedBalance != 0 ||
		snapshot.ConsumedTotal != 0 ||
		snapshot.RefundedTotal != 0 ||
		snapshot.GrantedTotal != 0
}

func applyLedgerDelta(account *billingschema.BillingAccount, userID int, delta int, operationID string, reasonCode string) error {
	if account == nil || delta == 0 {
		return nil
	}
	if delta < 0 {
		_, err := billingdomain.CreditAccount(billingdomain.CreditAccountParams{
			AccountID:      account.AccountID,
			Amount:         int64(-delta),
			IdempotencyKey: fmt.Sprintf("ledger-credit:%s", operationID),
			ReasonCode:     reasonCode,
			ReferenceType:  "user",
			ReferenceID:    fmt.Sprintf("%d", userID),
			OperatorType:   "ledger_sync",
			OperatorID:     operationID,
		})
		return err
	}

	reservation, err := billingdomain.CreateReservation(billingdomain.CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      ledgerSyncRequestID(operationID),
		ReservedAmount: int64(delta),
		IdempotencyKey: fmt.Sprintf("ledger-reservation:%s", operationID),
	})
	if err != nil {
		return err
	}
	_, err = billingdomain.SettleReservation(billingdomain.SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   int64(delta),
		IdempotencyKey: fmt.Sprintf("ledger-settlement:%s", operationID),
	})
	return err
}

func applyLedgerDeltaTx(tx *gorm.DB, account *billingschema.BillingAccount, userID int, delta int, operationID string, reasonCode string) error {
	if account == nil || delta == 0 {
		return nil
	}
	if delta < 0 {
		_, err := billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
			AccountID:      account.AccountID,
			Amount:         int64(-delta),
			IdempotencyKey: fmt.Sprintf("ledger-credit:%s", operationID),
			ReasonCode:     reasonCode,
			ReferenceType:  "user",
			ReferenceID:    fmt.Sprintf("%d", userID),
			OperatorType:   "ledger_sync",
			OperatorID:     operationID,
		})
		return err
	}

	reservation, err := billingdomain.CreateReservationTx(tx, billingdomain.CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      ledgerSyncRequestID(operationID),
		ReservedAmount: int64(delta),
		IdempotencyKey: fmt.Sprintf("ledger-reservation:%s", operationID),
	})
	if err != nil {
		return err
	}
	_, err = billingdomain.SettleReservationTx(tx, billingdomain.SettleReservationParams{
		ReservationID:  reservation.ReservationID,
		ActualAmount:   int64(delta),
		IdempotencyKey: fmt.Sprintf("ledger-settlement:%s", operationID),
	})
	return err
}

func ledgerSyncRequestID(operationID string) string {
	const prefix = "ledger-sync:"
	requestID := prefix + operationID
	if len(requestID) <= billingReservationRequestIDMax {
		return requestID
	}

	digest := sha256.Sum256([]byte(operationID))
	return prefix + hex.EncodeToString(digest[:24])
}

func getUserWalletQuotaTx(tx *gorm.DB, userID int) (int, error) {
	var quota int
	if err := tx.Model(&identityschema.User{}).Where("id = ?", userID).Select("quota").Find(&quota).Error; err != nil {
		return 0, err
	}
	return quota, nil
}

func increaseUserWalletQuotaTx(tx *gorm.DB, userID int, amount int) error {
	return tx.Model(&identityschema.User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", amount)).Error
}

func defaultReasonCode(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
