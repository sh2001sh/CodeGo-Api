package sessionstate

import (
	"errors"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	SecureVerificationSessionKey       = "secure_verified_at"
	SecureVerificationMethodSessionKey = "secure_verified_method"
	SecureVerificationMethod2FA        = "2fa"
	SecureVerificationMethodPasskey    = "passkey"
	PasskeyReadySessionKey             = "secure_passkey_ready_at"
	SecureVerificationTimeout          = 300
	PasskeyReadyTimeout                = 60
)

var (
	ErrSecureVerificationRequired       = errors.New("请先完成安全验证")
	ErrSecureVerificationMethodMismatch = errors.New("请先完成对应的安全验证")
	ErrInvalidPasskeyReadyState         = errors.New("无效的 Passkey 验证状态")
)

func SetSecureVerificationSession(c *gin.Context, method string) (int64, error) {
	session := sessions.Default(c)
	session.Delete(PasskeyReadySessionKey)
	now := time.Now().Unix()
	session.Set(SecureVerificationSessionKey, now)
	session.Set(SecureVerificationMethodSessionKey, method)
	if err := session.Save(); err != nil {
		return 0, err
	}
	return now, nil
}

func ConsumePasskeyReady(c *gin.Context) (bool, error) {
	session := sessions.Default(c)
	readyAtRaw := session.Get(PasskeyReadySessionKey)
	if readyAtRaw == nil {
		return false, nil
	}

	readyAt, ok := readyAtRaw.(int64)
	if !ok {
		session.Delete(PasskeyReadySessionKey)
		_ = session.Save()
		return false, ErrInvalidPasskeyReadyState
	}
	session.Delete(PasskeyReadySessionKey)
	if err := session.Save(); err != nil {
		return false, err
	}
	if time.Now().Unix()-readyAt >= PasskeyReadyTimeout {
		return false, nil
	}
	return true, nil
}

func MarkPasskeyReady(c *gin.Context) error {
	session := sessions.Default(c)
	session.Set(PasskeyReadySessionKey, time.Now().Unix())
	session.Delete(SecureVerificationSessionKey)
	session.Delete(SecureVerificationMethodSessionKey)
	return session.Save()
}

func RequireSecureVerificationMethod(c *gin.Context, method string) error {
	session := sessions.Default(c)
	verifiedAt, ok := session.Get(SecureVerificationSessionKey).(int64)
	if !ok || time.Now().Unix()-verifiedAt >= SecureVerificationTimeout {
		session.Delete(SecureVerificationSessionKey)
		session.Delete(SecureVerificationMethodSessionKey)
		_ = session.Save()
		return ErrSecureVerificationRequired
	}

	verifiedMethod, ok := session.Get(SecureVerificationMethodSessionKey).(string)
	if !ok || verifiedMethod != method {
		return ErrSecureVerificationMethodMismatch
	}
	return nil
}
