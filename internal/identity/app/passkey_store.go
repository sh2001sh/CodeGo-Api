package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	passkeycodec "github.com/sh2001sh/CodeGo-Api/internal/identity/app/passkey"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

func getPasskeyByUserID(userID int) (*identitydomain.PasskeyCredential, error) {
	if userID == 0 {
		platformobservability.SysLog("GetPasskeyByUserID: empty user ID")
		return nil, identitydomain.ErrFriendlyPasskeyNotFound
	}
	var credential identitydomain.PasskeyCredential
	if err := platformdb.DB.Where("user_id = ?", userID).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, identitydomain.ErrPasskeyNotFound
		}
		platformobservability.SysLog(fmt.Sprintf("GetPasskeyByUserID: database error for user %d: %v", userID, err))
		return nil, identitydomain.ErrFriendlyPasskeyNotFound
	}
	return &credential, nil
}

func getPasskeyByCredentialID(credentialID []byte) (*identitydomain.PasskeyCredential, error) {
	if len(credentialID) == 0 {
		platformobservability.SysLog("GetPasskeyByCredentialID: empty credential ID")
		return nil, identitydomain.ErrFriendlyPasskeyNotFound
	}

	credIDStr := base64.StdEncoding.EncodeToString(credentialID)
	var credential identitydomain.PasskeyCredential
	if err := platformdb.DB.Where("credential_id = ?", credIDStr).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			platformobservability.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: passkey not found for credential ID length %d", len(credentialID)))
			return nil, identitydomain.ErrFriendlyPasskeyNotFound
		}
		platformobservability.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: database error for credential ID: %v", err))
		return nil, identitydomain.ErrFriendlyPasskeyNotFound
	}
	return &credential, nil
}

func upsertPasskeyCredential(credential *identitydomain.PasskeyCredential) error {
	if credential == nil {
		platformobservability.SysLog("UpsertPasskeyCredential: nil credential provided")
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", credential.UserID).Delete(&identitydomain.PasskeyCredential{}).Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to delete existing credential for user %d: %v", credential.UserID, err))
			return fmt.Errorf("Passkey 保存失败，请重试")
		}
		if err := tx.Create(credential).Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to create credential for user %d: %v", credential.UserID, err))
			return fmt.Errorf("Passkey 保存失败，请重试")
		}
		return nil
	})
}

func deletePasskeyByUserID(userID int) error {
	if userID == 0 {
		platformobservability.SysLog("DeletePasskeyByUserID: empty user ID")
		return fmt.Errorf("删除失败，请重试")
	}
	if err := platformdb.DB.Unscoped().Where("user_id = ?", userID).Delete(&identitydomain.PasskeyCredential{}).Error; err != nil {
		platformobservability.SysLog(fmt.Sprintf("DeletePasskeyByUserID: failed to delete passkey for user %d: %v", userID, err))
		return fmt.Errorf("删除失败，请重试")
	}
	return nil
}

func newPasskeyCredentialFromWebAuthn(userID int, credential *webauthn.Credential) *identitydomain.PasskeyCredential {
	return passkeycodec.NewCredentialRecord(userID, credential)
}

func markPasskeyLastUsed(credential *identitydomain.PasskeyCredential) {
	if credential == nil {
		return
	}
	now := time.Now()
	credential.LastUsedAt = &now
}
