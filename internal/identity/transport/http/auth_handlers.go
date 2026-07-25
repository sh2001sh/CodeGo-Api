package http

import (
	"encoding/json"
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/sessionstate"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

func Login(c *gin.Context) {
	var req identityapp.LoginRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	result, err := identityapp.AuthenticatePasswordLogin(req)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	if result.RequireTwoFA {
		session := sessions.Default(c)
		session.Set(identityapp.PendingUsernameSessionKey, result.User.Username)
		session.Set(identityapp.PendingUserIDSessionKey, result.User.ID)
		if err := session.Save(); err != nil {
			handleAuthError(c, identityapp.ErrSessionSaveFailed)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": httpapi.TranslateMessage(c, i18n.MsgUserRequire2FA),
			"success": true,
			"data": map[string]any{
				"require_2fa": true,
			},
		})
		return
	}

	if err := establishAuthenticatedSession(c, sessions.Default(c), result.User); err != nil {
		handleAuthError(c, err)
		return
	}
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
	})
}

func Register(c *gin.Context) {
	var req identityapp.RegisterRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := identityapp.RegisterPasswordUser(req); err != nil {
		handleAuthError(c, err)
		return
	}

	user, err := identitystore.AuthenticateUserCredentials(req.Username, req.Password)
	if err != nil {
		handleAuthError(c, err)
		return
	}
	if err := establishAuthenticatedSession(c, sessions.Default(c), identityapp.BuildAuthenticatedSessionUser(user)); err != nil {
		handleAuthError(c, err)
	}
}

func establishAuthenticatedSession(c *gin.Context, session sessions.Session, user *identityapp.AuthenticatedSessionUser) error {
	if user == nil {
		return identityapp.ErrInvalidParams
	}
	if err := sessionstate.SaveAuthenticatedSession(c, &identityschema.User{
		Id:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Group:       user.Group,
	}); err != nil {
		return identityapp.ErrSessionSaveFailed
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
		},
	})
	return nil
}

func handleAuthError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, identityapp.ErrPasswordLoginDisabled),
		errors.Is(err, identityapp.ErrRegisterDisabled),
		errors.Is(err, identityapp.ErrPasswordRegisterDisabled),
		errors.Is(err, identityapp.ErrInvalidParams),
		errors.Is(err, identityapp.ErrUsernameOrPasswordError),
		errors.Is(err, identityapp.ErrEmailVerificationNeeded),
		errors.Is(err, identityapp.ErrVerificationCodeInvalid),
		errors.Is(err, identityapp.ErrUserExists),
		errors.Is(err, identityapp.ErrSessionSaveFailed),
		errors.Is(err, identityapp.ErrRegisterFailed),
		errors.Is(err, identityapp.ErrDefaultTokenFailed),
		errors.Is(err, identityapp.ErrCreateDefaultToken):
		httpapi.ApiErrorI18n(c, err.Error())
	case errors.Is(err, identityapp.ErrDatabaseError):
		httpapi.ApiErrorI18n(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
