package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterPlatformRoutes(apiRouter *gin.RouterGroup, anonymousRequestBodyLimit gin.HandlerFunc) {
	apiRouter.GET("/setup", GetSetup)
	apiRouter.POST("/setup", anonymousRequestBodyLimit, PostSetup)
	apiRouter.GET("/status", GetStatus)
	apiRouter.GET("/uptime/status", GetUptimeKumaStatus)
	apiRouter.GET("/status/test", middleware.AdminAuth(), TestStatus)
	apiRouter.GET("/notice", GetNotice)
	apiRouter.GET("/user-agreement", GetUserAgreement)
	apiRouter.GET("/privacy-policy", GetPrivacyPolicy)
	apiRouter.GET("/about", GetAbout)
	apiRouter.GET("/home_page_content", GetHomePageContent)
	apiRouter.GET("/home_page_packages_content", GetHomePagePackagesContent)
}
