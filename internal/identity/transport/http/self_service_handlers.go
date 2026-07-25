package http

import (
	"encoding/json"
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
)

// UpdateSelf handles authenticated self-service profile and preference mutations.
func UpdateSelf(c *gin.Context) {
	var requestData map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&requestData); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if sidebarModules, ok := requestData["sidebar_modules"]; ok {
		if err := identityapp.UpdateSelfSidebarModules(c.GetInt("id"), sidebarModules); err != nil {
			handleSelfServiceError(c, err)
			return
		}
		httpapi.ApiSuccessI18n(c, i18n.MsgUpdateSuccess, nil)
		return
	}

	if language, ok := requestData["language"]; ok {
		if err := identityapp.UpdateSelfLanguage(c.GetInt("id"), language); err != nil {
			handleSelfServiceError(c, err)
			return
		}
		httpapi.ApiSuccessI18n(c, i18n.MsgUpdateSuccess, nil)
		return
	}

	req, err := identityapp.DecodeUpdateSelfProfileRequest(requestData)
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := identityapp.UpdateSelfProfile(c.GetInt("id"), req); err != nil {
		handleSelfServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// DeleteSelf handles authenticated self-service account deletion.
func DeleteSelf(c *gin.Context) {
	if err := identityapp.DeleteSelf(c.GetInt("id")); err != nil {
		handleSelfServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GenerateAccessToken rotates the authenticated user's system access token.
func GenerateAccessToken(c *gin.Context) {
	token, err := identityapp.GenerateAccessToken(c.GetInt("id"))
	if err != nil {
		handleSelfServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    token,
	})
}

func handleSelfServiceError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, identityapp.ErrInvalidParams):
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
	case errors.Is(err, identityapp.ErrInvalidInput):
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidInput)
	case errors.Is(err, identityapp.ErrUpdateFailed):
		httpapi.ApiErrorI18n(c, i18n.MsgUpdateFailed)
	case errors.Is(err, identityapp.ErrGenerateAccessFailed):
		httpapi.ApiErrorI18n(c, i18n.MsgGenerateFailed)
	case errors.Is(err, identityapp.ErrUUIDDuplicate):
		httpapi.ApiErrorI18n(c, i18n.MsgUuidDuplicate)
	case errors.Is(err, identityapp.ErrCannotDeleteRootUser):
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
	case errors.Is(err, identityapp.ErrOriginalPassword):
		httpapi.ApiError(c, err)
	default:
		httpapi.ApiError(c, err)
	}
}
