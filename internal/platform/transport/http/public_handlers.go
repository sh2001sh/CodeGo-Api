package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	platformapp "github.com/sh2001sh/CodeGo-Api/internal/platform/app"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

// GetSetup returns the current setup state.
func GetSetup(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    platformapp.GetSetupStatus(),
	})
}

// PostSetup completes the initial instance setup flow.
func PostSetup(c *gin.Context) {
	var req platformapp.SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请求参数有误",
		})
		return
	}

	if err := platformapp.CompleteSetup(req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "系统初始化成功",
	})
}

// TestStatus returns a lightweight runtime health snapshot for admins.
func TestStatus(c *gin.Context) {
	if err := platformapp.CheckRuntimeHealth(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "数据库连接失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Server is running",
		"http_stats": middleware.GetStats(),
	})
}

// GetStatus returns public runtime and site metadata for the web shell.
func GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    platformapp.GetPublicStatus(),
	})
}

// GetUptimeKumaStatus returns public grouped Uptime Kuma monitor data.
func GetUptimeKumaStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    platformapp.GetUptimeStatus(c.Request.Context()),
	})
}

// GetNotice returns the public notice content.
func GetNotice(c *gin.Context) {
	writePublicString(c, platformapp.GetNotice())
}

// GetUserAgreement returns the current public user agreement.
func GetUserAgreement(c *gin.Context) {
	writePublicString(c, platformapp.GetUserAgreement())
}

// GetPrivacyPolicy returns the current public privacy policy.
func GetPrivacyPolicy(c *gin.Context) {
	writePublicString(c, platformapp.GetPrivacyPolicy())
}

// GetAbout returns the public about content.
func GetAbout(c *gin.Context) {
	writePublicString(c, platformapp.GetAbout())
}

// GetHomePageContent returns the public homepage content block.
func GetHomePageContent(c *gin.Context) {
	writePublicString(c, platformapp.GetHomePageContent())
}

// GetHomePagePackagesContent returns the public homepage packages content block.
func GetHomePagePackagesContent(c *gin.Context) {
	writePublicString(c, platformapp.GetHomePagePackagesContent())
}

func writePublicString(c *gin.Context, value string) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    value,
	})
}
