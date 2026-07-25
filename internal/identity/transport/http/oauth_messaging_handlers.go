package http

import (
	"encoding/json"
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
)

type weChatBindRequest struct {
	Code string `json:"code"`
}

// HandleWeChatOAuth handles login-or-register via WeChat.
func HandleWeChatOAuth(c *gin.Context) {
	user, err := identityapp.CompleteWeChatLogin(c.Request.Context(), c.Query("code"))
	if err != nil {
		handleMessagingOAuthError(c, err)
		return
	}
	if err := establishAuthenticatedSession(c, sessions.Default(c), user); err != nil {
		handleAuthError(c, err)
	}
}

// BindWeChatOAuth binds a WeChat account to the current session user.
func BindWeChatOAuth(c *gin.Context) {
	var req weChatBindRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrMessagingOAuthInvalid.Error())
		return
	}

	session := sessions.Default(c)
	userID, _ := session.Get("id").(int)
	if err := identityapp.BindWeChatAccount(c.Request.Context(), userID, req.Code); err != nil {
		handleMessagingOAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// HandleTelegramLogin handles Telegram login.
func HandleTelegramLogin(c *gin.Context) {
	user, err := identityapp.CompleteTelegramLogin(c.Request.URL.Query())
	if err != nil {
		handleMessagingOAuthError(c, err)
		return
	}
	if err := establishAuthenticatedSession(c, sessions.Default(c), user); err != nil {
		handleAuthError(c, err)
	}
}

// BindTelegramOAuth binds Telegram to the current session user and preserves redirect behavior.
func BindTelegramOAuth(c *gin.Context) {
	session := sessions.Default(c)
	userID, _ := session.Get("id").(int)
	if err := identityapp.BindTelegramAccount(userID, c.Request.URL.Query()); err != nil {
		handleMessagingOAuthError(c, err)
		return
	}
	c.Redirect(http.StatusFound, platformconfig.ThemeAwarePath("/console/personal"))
}

func handleMessagingOAuthError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, identityapp.ErrWeChatAuthDisabled),
		errors.Is(err, identityapp.ErrTelegramOAuthDisabled),
		errors.Is(err, identityapp.ErrMessagingOAuthInvalid),
		errors.Is(err, identityapp.ErrWeChatAlreadyBound),
		errors.Is(err, identityapp.ErrTelegramAlreadyBound),
		errors.Is(err, identityapp.ErrMessagingUserDeleted),
		errors.Is(err, identityapp.ErrMessagingRegisterDisabled),
		errors.Is(err, identityapp.ErrMessagingUserBanned):
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
