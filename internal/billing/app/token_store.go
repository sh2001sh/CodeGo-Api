package app

import (
	"errors"
	"fmt"

	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"strings"

	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

type TokenSnapshot struct {
	ID             int
	Key            string
	ExpiredTime    int64
	RemainQuota    int
	UsedQuota      int
	UnlimitedQuota bool
}

func GetTokenByKey(tokenKey string) (*TokenSnapshot, error) {
	token, err := identitystore.LoadTokenByKey(strings.TrimSpace(tokenKey), false)
	if err != nil {
		return nil, err
	}
	snapshot := tokenSnapshotFromModel(token)
	if snapshot.UnlimitedQuota {
		return snapshot, nil
	}
	remainQuota, err := getLedgerBackedTokenQuota(token)
	if err != nil {
		return nil, err
	}
	snapshot.RemainQuota = remainQuota
	return snapshot, nil
}

func GetTokenByID(tokenID int) (*TokenSnapshot, error) {
	if tokenID <= 0 {
		return nil, errors.New("id 为空！")
	}
	token := &identityschema.Token{Id: tokenID}
	err := platformdb.DB.First(token, "id = ?", tokenID).Error
	if err != nil {
		return nil, err
	}
	snapshot := tokenSnapshotFromModel(token)
	if snapshot.UnlimitedQuota {
		return snapshot, nil
	}
	remainQuota, err := getLedgerBackedTokenQuota(token)
	if err != nil {
		return nil, err
	}
	snapshot.RemainQuota = remainQuota
	return snapshot, nil
}

func GetUserUsedQuota(userID int) (int, error) {
	var quota int
	err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Select("used_quota").Find(&quota).Error
	return quota, err
}

func AdjustTokenQuota(tokenID int, tokenKey string, delta int) error {
	if tokenID <= 0 || delta == 0 {
		return nil
	}
	token, err := loadTokenForLedger(tokenID)
	if err != nil {
		return err
	}
	if token.UnlimitedQuota {
		return nil
	}
	account, err := ensureMirroredTokenAccount(token)
	if err != nil {
		return err
	}
	if err := reconcileTokenAccountBalance(account, token); err != nil {
		return err
	}

	operationID := platformruntime.GetUUID()
	if err := applyTokenLedgerDelta(account, token.Id, delta, operationID); err != nil {
		return err
	}
	if err := identitystore.AdjustTokenQuota(tokenID, strings.TrimSpace(tokenKey), delta); err != nil {
		compensationErr := applyTokenLedgerDelta(account, token.Id, -delta, platformruntime.GetUUID())
		if compensationErr != nil {
			return errors.Join(err, fmt.Errorf("token ledger compensation failed: %w", compensationErr))
		}
		return err
	}
	return nil
}

func getLedgerBackedTokenQuota(token *identityschema.Token) (int, error) {
	if token == nil || token.Id <= 0 {
		return 0, errors.New("invalid token")
	}
	var account billingschema.BillingAccount
	err := platformdb.DB.Where("owner_type = ? AND owner_id = ? AND account_type = ? AND quota_unit = ?", "token", token.Id, "token", "quota").First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || isMissingBillingSchema(err) {
		return token.RemainQuota, nil
	}
	if err != nil {
		return 0, err
	}
	snapshot, err := loadBalanceSnapshot(account.AccountID)
	if err != nil {
		return 0, err
	}
	return int(snapshot.AvailableBalance), nil
}

func loadTokenForLedger(tokenID int) (*identityschema.Token, error) {
	var token identityschema.Token
	if err := platformdb.DB.Where("id = ?", tokenID).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func ensureMirroredTokenAccount(token *identityschema.Token) (*billingschema.BillingAccount, error) {
	if token == nil || token.Id <= 0 {
		return nil, errors.New("invalid token")
	}
	account, err := billingdomain.EnsureBillingAccount(billingdomain.EnsureAccountParams{
		AccountType: "token", OwnerType: "token", OwnerID: int64(token.Id), QuotaUnit: "quota",
	})
	if err != nil {
		return nil, err
	}
	snapshot, err := loadBalanceSnapshot(account.AccountID)
	if err != nil {
		return nil, err
	}
	if hasNonZeroSnapshot(snapshot) || token.RemainQuota <= 0 {
		return account, nil
	}
	_, err = billingdomain.CreditAccount(billingdomain.CreditAccountParams{
		AccountID: account.AccountID, Amount: int64(token.RemainQuota),
		IdempotencyKey: fmt.Sprintf("mirror-bootstrap:token:%d", token.Id), ReasonCode: "mirror_bootstrap",
		ReferenceType: "token", ReferenceID: fmt.Sprintf("%d", token.Id), OperatorType: "ledger_sync", OperatorID: "mirror_bootstrap",
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func reconcileTokenAccountBalance(account *billingschema.BillingAccount, token *identityschema.Token) error {
	if account == nil || token == nil {
		return errors.New("token account is required")
	}
	snapshot, err := loadBalanceSnapshot(account.AccountID)
	if err != nil {
		return err
	}
	diff := token.RemainQuota - int(snapshot.AvailableBalance)
	if diff == 0 {
		return nil
	}
	return applyTokenLedgerDelta(account, token.Id, -diff, platformruntime.GetUUID())
}

func applyTokenLedgerDelta(account *billingschema.BillingAccount, tokenID int, delta int, operationID string) error {
	if account == nil || delta == 0 {
		return nil
	}
	if delta < 0 {
		_, err := billingdomain.CreditAccount(billingdomain.CreditAccountParams{
			AccountID: account.AccountID, Amount: int64(-delta), IdempotencyKey: "token-credit:" + operationID,
			ReasonCode: "token_quota_adjustment", ReferenceType: "token", ReferenceID: fmt.Sprintf("%d", tokenID),
			OperatorType: "ledger_sync", OperatorID: operationID,
		})
		return err
	}
	reservation, err := billingdomain.CreateReservation(billingdomain.CreateReservationParams{
		AccountID: account.AccountID, RequestID: "token-sync:" + operationID, ReservedAmount: int64(delta), IdempotencyKey: "token-reservation:" + operationID,
	})
	if err != nil {
		return err
	}
	_, err = billingdomain.SettleReservation(billingdomain.SettleReservationParams{
		ReservationID: reservation.ReservationID, ActualAmount: int64(delta), IdempotencyKey: "token-settlement:" + operationID,
	})
	return err
}

func tokenSnapshotFromModel(token *identityschema.Token) *TokenSnapshot {
	if token == nil {
		return nil
	}
	return &TokenSnapshot{
		ID:             token.Id,
		Key:            token.Key,
		ExpiredTime:    token.ExpiredTime,
		RemainQuota:    token.RemainQuota,
		UsedQuota:      token.UsedQuota,
		UnlimitedQuota: token.UnlimitedQuota,
	}
}
