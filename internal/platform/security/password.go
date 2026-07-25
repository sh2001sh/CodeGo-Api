package security

import "golang.org/x/crypto/bcrypt"

// Password2Hash hashes a plaintext password with the default bcrypt cost.
func Password2Hash(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashedPassword), err
}

// ValidatePasswordAndHash reports whether the plaintext password matches the bcrypt hash.
func ValidatePasswordAndHash(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
