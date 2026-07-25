package runtime

import (
	crand "crypto/rand"
	"encoding/base64"
	"math/big"
	"math/rand"

	"github.com/samber/lo"
)

const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateRandomCharsKey returns a crypto-random alphanumeric string with the requested length.
func GenerateRandomCharsKey(length int) (string, error) {
	b := make([]byte, length)
	maxI := big.NewInt(int64(len(keyChars)))

	for i := range b {
		n, err := crand.Int(crand.Reader, maxI)
		if err != nil {
			return "", err
		}
		b[i] = keyChars[n.Int64()]
	}

	return string(b), nil
}

// GenerateRandomKey returns a base64-encoded crypto-random key near the requested output length.
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length*3/4)
	if _, err := crand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// GenerateKey returns a 48-character crypto-random alphanumeric key.
func GenerateKey() (string, error) {
	return GenerateRandomCharsKey(48)
}

// GetRandomInt returns a pseudo-random integer in [0, max).
func GetRandomInt(max int) int {
	return rand.Intn(max)
}

// GetRandomString returns a pseudo-random alphanumeric string with the requested length.
func GetRandomString(length int) string {
	if length <= 0 {
		return ""
	}
	return lo.RandomString(length, lo.AlphanumericCharset)
}
