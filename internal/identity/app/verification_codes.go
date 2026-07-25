package app

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// EmailVerificationPurpose identifies codes used for email ownership checks.
	EmailVerificationPurpose = "v"
	// PasswordResetPurpose identifies codes used for password-reset links.
	PasswordResetPurpose = "r"
)

type verificationValue struct {
	code string
	time time.Time
}

var verificationMutex sync.Mutex
var verificationMap = make(map[string]verificationValue)
var verificationMapMaxSize = 10

// VerificationValidMinutes defines how long a verification code remains valid.
var VerificationValidMinutes = 10

// GenerateVerificationCode returns a UUID-derived verification token or its prefix.
func GenerateVerificationCode(length int) string {
	code := strings.ReplaceAll(uuid.New().String(), "-", "")
	if length == 0 {
		return code
	}
	return code[:length]
}

// RegisterVerificationCodeWithKey stores a verification code under a purpose-qualified key.
func RegisterVerificationCodeWithKey(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()

	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredVerificationPairs()
	}
}

// VerifyCodeWithKey reports whether a stored verification code matches and is still valid.
func VerifyCodeWithKey(key string, code string, purpose string) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()

	value, ok := verificationMap[purpose+key]
	now := time.Now()
	if !ok || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	return code == value.code
}

// DeleteVerificationKey removes a purpose-qualified verification code entry.
func DeleteVerificationKey(key string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	delete(verificationMap, purpose+key)
}

func removeExpiredVerificationPairs() {
	now := time.Now()
	for key := range verificationMap {
		if int(now.Sub(verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(verificationMap, key)
		}
	}
}
