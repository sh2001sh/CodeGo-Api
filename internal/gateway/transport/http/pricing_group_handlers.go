package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func GetPricing(c *gin.Context) {
	userID := c.GetInt("id")
	_, hasUser := c.Get("id")

	payload := gatewayroutingapp.BuildPricingPayload(userID, hasUser)
	c.JSON(stdhttp.StatusOK, gin.H{
		"success":            true,
		"data":               payload.Data,
		"vendors":            payload.Vendors,
		"group_ratio":        payload.GroupRatio,
		"usable_group":       payload.UsableGroup,
		"supported_endpoint": payload.SupportedEndpoint,
		"auto_groups":        payload.AutoGroups,
		"pricing_version":    payload.PricingVersion,
	})
}

func ResetModelRatio(c *gin.Context) {
	if err := gatewayroutingapp.ResetModelRatio(); err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}

func GetGroups(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gatewayroutingapp.BuildAllGroupNames(),
	})
}

func GetUserGroups(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gatewayroutingapp.BuildUserGroupsPayload(c.GetInt("id")),
	})
}

func GetRatioConfig(c *gin.Context) {
	data, ok := gatewayroutingapp.ExposedRatioConfig()
	if !ok {
		c.JSON(stdhttp.StatusForbidden, gin.H{
			"success": false,
			"message": "倍率配置接口未启用",
		})
		return
	}
	httpapi.ApiSuccess(c, data)
}
