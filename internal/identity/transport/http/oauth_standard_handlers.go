package http

import (
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
)

// GenerateOAuthCode creates and persists the OAuth state token.
func GenerateOAuthCode(c *gin.Context) {
	session := sessions.Default(c)
	state := identityapp.GenerateOAuthState()
	session.Set("oauth_state", state)
	if err := session.Save(); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, state)
}

// HandleOAuth handles the standard OAuth callback flow.
func HandleOAuth(c *gin.Context) {
	provider, err := identityapp.ResolveOAuthProvider(c.Param("provider"))
	if err != nil {
		handleStandardOAuthError(c, err)
		return
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get("oauth_state").(string)
	if err := identityapp.ValidateOAuthState(expectedState, c.Query("state")); err != nil {
		handleStandardOAuthError(c, err)
		return
	}

	if errorCode := c.Query("error"); errorCode != "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": c.Query("error_description"),
		})
		return
	}

	sessionUserID := 0
	isBind := false
	if session.Get("username") != nil {
		isBind = true
		sessionUserID, _ = session.Get("id").(int)
	}
	result, err := identityapp.CompleteOAuthFlow(
		c,
		provider,
		c.Query("code"),
		isBind,
		sessionUserID,
	)
	if err != nil {
		handleStandardOAuthError(c, err)
		return
	}

	if result.Action == "bind" {
		httpapi.ApiSuccessI18n(c, i18n.MsgOAuthBindSuccess, gin.H{"action": "bind"})
		return
	}

	user := result.User
	if user == nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := establishAuthenticatedSession(c, session, user); err != nil {
		handleAuthError(c, err)
		return
	}
}

func handleStandardOAuthError(c *gin.Context, err error) {
	var unknownProviderErr *identityapp.OAuthUnknownProviderError
	var invalidStateErr *identityapp.OAuthStateInvalidError
	var deletedUserErr *identityapp.OAuthUserDeletedError
	var registrationDisabledErr *identityapp.OAuthRegistrationDisabledError

	switch e := err.(type) {
	case *oauth.OAuthError:
		if e.Params != nil {
			httpapi.ApiErrorI18n(c, e.MsgKey, e.Params)
		} else {
			httpapi.ApiErrorI18n(c, e.MsgKey)
		}
	case *oauth.AccessDeniedError:
		httpapi.ApiErrorMsg(c, e.Message)
	case *oauth.TrustLevelError:
		httpapi.ApiErrorI18n(c, i18n.MsgOAuthTrustLevelLow)
	default:
		switch {
		case errors.Is(err, identityapp.ErrOAuthProviderDisabled):
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthNotEnabled, map[string]any{"Provider": providerDisplayName(c)})
		case errors.As(err, &unknownProviderErr):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthUnknownProvider)})
		case errors.As(err, &invalidStateErr):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthStateInvalid)})
		case errors.As(err, &deletedUserErr):
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserDeleted)
		case errors.As(err, &registrationDisabledErr):
			httpapi.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		case errors.Is(err, identityapp.ErrOAuthUserBanned):
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthUserBanned)
		case errors.Is(err, identityapp.ErrOAuthAlreadyBound):
			httpapi.ApiErrorI18n(c, i18n.MsgOAuthAlreadyBound, map[string]any{"Provider": providerDisplayName(c)})
		default:
			httpapi.ApiError(c, err)
		}
	}
}

func providerDisplayName(c *gin.Context) string {
	provider, err := identityapp.ResolveOAuthProvider(c.Param("provider"))
	if err != nil {
		return c.Param("provider")
	}
	return provider.GetName()
}
