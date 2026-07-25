package http

import (
	stdhttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func GetChannelAffinityCacheStats(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gatewayroutingapp.BuildChannelAffinityCacheStats(),
	})
}

func ClearChannelAffinityCache(c *gin.Context) {
	all := strings.TrimSpace(c.Query("all")) == "true"
	ruleName := strings.TrimSpace(c.Query("rule_name"))

	if !all && ruleName == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少参数：rule_name，或使用 all=true 清空全部",
		})
		return
	}

	deleted, err := gatewayroutingapp.ClearChannelAffinityCache(all, ruleName)
	if err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"deleted": deleted,
		},
	})
}

func GetChannelAffinityUsageCacheStats(c *gin.Context) {
	ruleName := strings.TrimSpace(c.Query("rule_name"))
	usingGroup := strings.TrimSpace(c.Query("using_group"))
	keyFingerprint := strings.TrimSpace(c.Query("key_fp"))

	if ruleName == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "missing param: rule_name",
		})
		return
	}
	if keyFingerprint == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "missing param: key_fp",
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gatewayroutingapp.BuildChannelAffinityUsageCacheStats(ruleName, usingGroup, keyFingerprint),
	})
}
