package domain

import "errors"

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserEmptyCredentials = errors.New("empty credentials")
	ErrTokenNotProvided     = errors.New("token not provided")
	ErrTokenInvalid         = errors.New("token invalid")
	ErrTwoFANotEnabled      = errors.New("2fa not enabled")
)
