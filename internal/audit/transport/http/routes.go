package http

import (
	"github.com/gin-gonic/gin"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterAuditRoutes(apiRouter *gin.RouterGroup) {
	logRoute := apiRouter.Group("/log")
	logRoute.GET("/", middleware.AdminAuth(), GetAllLogs)
	logRoute.DELETE("/", middleware.AdminAuth(), DeleteHistoryLogs)
	logRoute.GET("/stat", middleware.AdminAuth(), GetLogsStat)
	logRoute.GET("/self/stat", middleware.UserAuth(), GetLogsSelfStat)
	logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), gatewayhttp.GetChannelAffinityUsageCacheStats)
	logRoute.GET("/search", middleware.AdminAuth(), SearchAllLogs)
	logRoute.GET("/self", middleware.UserAuth(), GetUserLogs)
	logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), SearchUserLogs)
	logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
	{
		logRoute.GET("/token", middleware.TokenAuthReadOnly(), GetLogByKey)
	}

	dataRoute := apiRouter.Group("/data")
	dataRoute.GET("/", middleware.AdminAuth(), GetAllQuotaDates)
	dataRoute.GET("/users", middleware.AdminAuth(), GetQuotaDatesByUser)
	dataRoute.GET("/self", middleware.UserAuth(), GetUserQuotaDates)
}
