package app

import (
	"errors"
	"fmt"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"strconv"
	"time"
)

var (
	ErrPasskeyNotLoggedIn      = errors.New("未登录")
	ErrPasskeyInvalidSession   = errors.New("无效的会话信息")
	ErrPasskeyUserDisabled     = errors.New("该用户已被禁用")
	ErrPasskeyInvalidUserID    = errors.New("无效的用户 ID")
	ErrPasskeyNotBound         = errors.New("该用户尚未绑定 Passkey")
	ErrPasskeyLoginState       = errors.New("Passkey 登录状态异常")
	ErrPasskeyCredentialCreate = errors.New("无法创建 Passkey 凭证")
	ErrPasskeyCredentialUpdate = errors.New("Passkey 凭证更新失败")
)

type PasskeyStatusResponse struct {
	Enabled    bool       `json:"enabled"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// LoadActiveUser returns an enabled user suitable for passkey flows.
func LoadActiveUser(userID int) (*identityschema.User, error) {
	if userID <= 0 {
		return nil, ErrPasskeyNotLoggedIn
	}
	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return nil, err
	}
	if user.Status != constant.UserStatusEnabled {
		return nil, ErrPasskeyUserDisabled
	}
	return user, nil
}

// LoadOptionalPasskeyCredential returns the user's passkey credential if present.
func LoadOptionalPasskeyCredential(userID int) (*identitydomain.PasskeyCredential, error) {
	credential, err := getPasskeyByUserID(userID)
	if errors.Is(err, identitydomain.ErrPasskeyNotFound) {
		return nil, nil
	}
	return credential, err
}

// HasPasskeyCredential reports whether the user currently has a bound passkey credential.
func HasPasskeyCredential(userID int) (bool, error) {
	credential, err := LoadOptionalPasskeyCredential(userID)
	if err != nil {
		return false, err
	}
	return credential != nil, nil
}

// LoadPasskeyStatus returns the current binding status for the user.
func LoadPasskeyStatus(userID int) (*PasskeyStatusResponse, error) {
	credential, err := LoadOptionalPasskeyCredential(userID)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return &PasskeyStatusResponse{Enabled: false}, nil
	}
	return &PasskeyStatusResponse{
		Enabled:    true,
		LastUsedAt: credential.LastUsedAt,
	}, nil
}

// StoreRegisteredPasskeyCredential persists a newly registered credential for the user.
func StoreRegisteredPasskeyCredential(userID int, credential *webauthn.Credential) error {
	passkeyCredential := newPasskeyCredentialFromWebAuthn(userID, credential)
	if passkeyCredential == nil {
		return ErrPasskeyCredentialCreate
	}
	return upsertPasskeyCredential(passkeyCredential)
}

// ResolvePasskeyLoginUser locates the enabled user for a validated passkey login ceremony.
func ResolvePasskeyLoginUser(rawID []byte, userHandle []byte) (*identityschema.User, *identitydomain.PasskeyCredential, error) {
	credential, err := getPasskeyByCredentialID(rawID)
	if err != nil {
		return nil, nil, fmt.Errorf("未找到 Passkey 凭证: %w", err)
	}

	user, err := identitystore.LoadUserByID(credential.UserID, true)
	if err != nil {
		return nil, nil, fmt.Errorf("用户信息获取失败: %w", err)
	}
	if user.Status != constant.UserStatusEnabled {
		return nil, nil, ErrPasskeyUserDisabled
	}

	if len(userHandle) > 0 {
		userID, parseErr := strconv.Atoi(string(userHandle))
		if parseErr != nil {
			platformobservability.SysLog(fmt.Sprintf("PasskeyLogin: userHandle parse error for credential, length: %d", len(userHandle)))
		} else if userID != user.Id {
			return nil, nil, errors.New("用户句柄与凭证不匹配")
		}
	}

	return user, credential, nil
}

// StoreValidatedLoginCredential replaces the user's stored credential after a successful passkey login.
func StoreValidatedLoginCredential(userID int, credential *webauthn.Credential) error {
	updatedCredential := newPasskeyCredentialFromWebAuthn(userID, credential)
	if updatedCredential == nil {
		return ErrPasskeyCredentialUpdate
	}
	markPasskeyLastUsed(updatedCredential)
	return upsertPasskeyCredential(updatedCredential)
}

// MarkPasskeyCredentialUsed updates last-used time for an existing credential record.
func MarkPasskeyCredentialUsed(credential *identitydomain.PasskeyCredential) error {
	if credential == nil {
		return ErrPasskeyNotBound
	}
	markPasskeyLastUsed(credential)
	return upsertPasskeyCredential(credential)
}

// DeletePasskeyBinding removes the passkey credential bound to the user.
func DeletePasskeyBinding(userID int) error {
	return deletePasskeyByUserID(userID)
}

// ResetPasskeyBinding validates the target user and clears their passkey binding.
func ResetPasskeyBinding(userID int) error {
	if userID <= 0 {
		return ErrPasskeyInvalidUserID
	}
	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return err
	}
	if _, err := getPasskeyByUserID(user.Id); err != nil {
		if errors.Is(err, identitydomain.ErrPasskeyNotFound) {
			return ErrPasskeyNotBound
		}
		return err
	}
	return deletePasskeyByUserID(user.Id)
}
