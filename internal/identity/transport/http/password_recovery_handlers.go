package http

import (
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
)

// SendEmailVerification issues a registration verification code to the requested email address.
func SendEmailVerification(c *gin.Context) {
	if err := identityapp.SendRegistrationVerification(c.Query("email")); err != nil {
		handlePasswordRecoveryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// SendPasswordResetEmail issues a password-reset email when the address exists.
func SendPasswordResetEmail(c *gin.Context) {
	if err := identityapp.SendPasswordResetEmail(c.Request.Context(), c.Query("email")); err != nil {
		handlePasswordRecoveryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// ResetPassword completes a password-reset flow and returns the generated temporary password.
func ResetPassword(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	req, err := identityapp.DecodePasswordResetRequest(body)
	if err != nil {
		handlePasswordRecoveryError(c, err)
		return
	}

	password, err := identityapp.ResetPassword(req)
	if err != nil {
		handlePasswordRecoveryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
}

func handlePasswordRecoveryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identityapp.ErrInvalidParams), errors.Is(err, identityapp.ErrInvalidEmailParameter):
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
	case errors.Is(err, identityapp.ErrInvalidEmailAddress):
		httpapi.ApiError(c, err)
	case errors.Is(err, identityapp.ErrEmailAlreadyTaken):
		httpapi.ApiError(c, err)
	case errors.Is(err, identityapp.ErrEmailDomainNotAllowed):
		httpapi.ApiError(c, err)
	case errors.Is(err, identityapp.ErrEmailAliasNotAllowed):
		httpapi.ApiError(c, err)
	case errors.Is(err, identityapp.ErrPasswordResetLinkInvalid):
		httpapi.ApiError(c, err)
	default:
		httpapi.ApiError(c, err)
	}
}
