package app

import (
	"errors"

	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

var ErrInvalidRequestBody = errors.New("invalid request body")

type EmailBindRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// BindEmail verifies the email code and persists the user's bound email address.
func BindEmail(userID int, email string, code string) error {
	if !VerifyCodeWithKey(email, code, EmailVerificationPurpose) {
		return ErrVerificationCodeInvalid
	}

	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return err
	}
	user.Email = email
	return identitystore.UpdateUser(user, false)
}
