package http

import (
	"github.com/gin-gonic/gin"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
)

func GetUserSelf(c *gin.Context) {
	payload, err := identityapp.GetSelfProfile(c.GetInt("id"), c.GetInt("role"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func GetUserModels(c *gin.Context) {
	models, err := identityapp.ListUserModelsForGroup(c.GetInt("id"), c.Query("group"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, models)
}
