package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
)

// GetUserOAuthBindings returns the current user's custom OAuth bindings.
func GetUserOAuthBindings(c *gin.Context) {
	response, err := identityapp.ListUserOAuthBindings(c.GetInt("id"))
	if err != nil {
		handleUserOAuthBindingError(c, err)
		return
	}
	httpapi.ApiSuccess(c, response)
}

// GetUserOAuthBindingsByAdmin returns a target user's custom OAuth bindings.
func GetUserOAuthBindingsByAdmin(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, "invalid user id")
		return
	}

	response, err := identityapp.ListUserOAuthBindingsAsAdmin(userID, c.GetInt("role"))
	if err != nil {
		handleUserOAuthBindingError(c, err)
		return
	}
	httpapi.ApiSuccess(c, response)
}

// UnbindCustomOAuth removes the current user's binding to a provider.
func UnbindCustomOAuth(c *gin.Context) {
	userID := c.GetInt("id")
	if userID == 0 {
		httpapi.ApiErrorMsg(c, identityapp.ErrCustomOAuthNotLoggedIn.Error())
		return
	}

	providerID, err := strconv.Atoi(c.Param("provider_id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, "无效的提供商 ID")
		return
	}

	if err := identityapp.UnbindUserOAuth(userID, providerID); err != nil {
		handleUserOAuthBindingError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "解绑成功",
	})
}

// UnbindCustomOAuthByAdmin removes a target user's provider binding.
func UnbindCustomOAuthByAdmin(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, "invalid user id")
		return
	}

	providerID, err := strconv.Atoi(c.Param("provider_id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, "invalid provider id")
		return
	}

	if err := identityapp.UnbindUserOAuthAsAdmin(userID, providerID, c.GetInt("role")); err != nil {
		handleUserOAuthBindingError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "success",
	})
}

func handleUserOAuthBindingError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case identityapp.ErrCustomOAuthNotLoggedIn, identityapp.ErrCustomOAuthNoPermission:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
