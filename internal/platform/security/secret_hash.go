package security

import platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"

// GenerateHMAC hashes data with the platform crypto secret.
func GenerateHMAC(data string) string {
	return HmacSha256(data, platformconfig.CryptoSecret)
}
