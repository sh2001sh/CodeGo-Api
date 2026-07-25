package security

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// BackupCodeLength defines the normalized backup-code size without separators.
	BackupCodeLength = 8
	// BackupCodeCount defines how many backup codes are generated per setup.
	BackupCodeCount = 4
	// MaxFailAttempts defines how many invalid attempts trigger a temporary lock.
	MaxFailAttempts = 5
	// LockoutDuration defines the lock duration in seconds after too many failures.
	LockoutDuration = 300
)

const defaultTwoFAIssuer = "codego-api"

// GenerateTOTPSecret creates a new TOTP secret for the supplied account name.
func GenerateTOTPSecret(accountName string, systemName string) (*otp.Key, error) {
	issuer := twoFAIssuer(systemName)
	return totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
}

// ValidateTOTPCode validates a six-digit TOTP code against the secret.
func ValidateTOTPCode(secret string, code string) bool {
	cleanCode := strings.ReplaceAll(code, " ", "")
	if len(cleanCode) != 6 {
		return false
	}
	return totp.Validate(cleanCode, secret)
}

// GenerateBackupCodes returns a fresh set of human-readable recovery codes.
func GenerateBackupCodes() ([]string, error) {
	codes := make([]string, BackupCodeCount)
	for i := 0; i < BackupCodeCount; i++ {
		code, err := generateRandomBackupCode()
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// ValidateBackupCode reports whether the backup code matches the supported format.
func ValidateBackupCode(code string) bool {
	cleanCode := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	if len(cleanCode) != BackupCodeLength {
		return false
	}
	for _, char := range cleanCode {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}
	return true
}

// NormalizeBackupCode normalizes a backup code to XXXX-XXXX uppercase form.
func NormalizeBackupCode(code string) string {
	cleanCode := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	if len(cleanCode) == BackupCodeLength {
		return fmt.Sprintf("%s-%s", cleanCode[:4], cleanCode[4:])
	}
	return code
}

// HashBackupCode hashes a normalized backup code for persistence.
func HashBackupCode(code string) (string, error) {
	return Password2Hash(NormalizeBackupCode(code))
}

// ValidateNumericCode validates a six-digit numeric verification code.
func ValidateNumericCode(code string) (string, error) {
	code = strings.ReplaceAll(code, " ", "")
	if len(code) != 6 {
		return "", fmt.Errorf("验证码必须是6位数字")
	}
	if _, err := strconv.Atoi(code); err != nil {
		return "", fmt.Errorf("验证码只能包含数字")
	}
	return code, nil
}

// GenerateQRCodeData builds an otpauth URI for QR-code rendering.
func GenerateQRCodeData(secret string, username string, systemName string) string {
	issuer := twoFAIssuer(systemName)
	accountName := fmt.Sprintf("%s (%s)", username, issuer)
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=6&period=30",
		issuer,
		accountName,
		secret,
		issuer,
	)
}

func twoFAIssuer(systemName string) string {
	systemName = strings.TrimSpace(systemName)
	if systemName == "" {
		return defaultTwoFAIssuer
	}
	return systemName
}

func generateRandomBackupCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, BackupCodeLength)
	for i := range code {
		randomBytes := make([]byte, 1)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", err
		}
		code[i] = charset[int(randomBytes[0])%len(charset)]
	}
	return fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:])), nil
}
