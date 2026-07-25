package http

import (
	"github.com/gin-gonic/gin"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterAdminOpsRoutes(apiRouter *gin.RouterGroup) {
	optionRoute := apiRouter.Group("/option")
	optionRoute.Use(middleware.RootAuth())
	{
		optionRoute.GET("/", GetOptions)
		optionRoute.PUT("/", UpdateOption)
		optionRoute.POST("/payment_compliance", ConfirmPaymentCompliance)
		optionRoute.GET("/channel_affinity_cache", gatewayhttp.GetChannelAffinityCacheStats)
		optionRoute.DELETE("/channel_affinity_cache", gatewayhttp.ClearChannelAffinityCache)
		optionRoute.POST("/rest_model_ratio", gatewayhttp.ResetModelRatio)
		optionRoute.POST("/migrate_console_setting", MigrateConsoleSetting)
	}

	customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
	customOAuthRoute.Use(middleware.RootAuth())
	{
		customOAuthRoute.POST("/discovery", FetchCustomOAuthDiscovery)
		customOAuthRoute.GET("/", GetCustomOAuthProviders)
		customOAuthRoute.GET("/:id", GetCustomOAuthProvider)
		customOAuthRoute.POST("/", CreateCustomOAuthProvider)
		customOAuthRoute.PUT("/:id", UpdateCustomOAuthProvider)
		customOAuthRoute.DELETE("/:id", DeleteCustomOAuthProvider)
	}

	performanceRoute := apiRouter.Group("/performance")
	performanceRoute.Use(middleware.RootAuth())
	{
		performanceRoute.GET("/stats", GetPerformanceStats)
		performanceRoute.DELETE("/disk_cache", ClearDiskCache)
		performanceRoute.POST("/reset_stats", ResetPerformanceStats)
		performanceRoute.POST("/gc", ForceGC)
		performanceRoute.GET("/logs", GetLogFiles)
		performanceRoute.DELETE("/logs", CleanupLogFiles)
	}

	ratioSyncRoute := apiRouter.Group("/ratio_sync")
	ratioSyncRoute.Use(middleware.RootAuth())
	{
		ratioSyncRoute.GET("/channels", GetSyncableChannels)
		ratioSyncRoute.POST("/fetch", FetchUpstreamRatios)
	}

	redemptionRoute := apiRouter.Group("/redemption")
	redemptionRoute.Use(middleware.AdminAuth())
	{
		redemptionRoute.GET("/", GetAllRedemptions)
		redemptionRoute.GET("/search", SearchRedemptions)
		redemptionRoute.GET("/:id", GetRedemption)
		redemptionRoute.POST("/", AddRedemption)
		redemptionRoute.PUT("/", UpdateRedemption)
		redemptionRoute.DELETE("/invalid", DeleteInvalidRedemption)
		redemptionRoute.DELETE("/:id", DeleteRedemption)
	}

	groupRoute := apiRouter.Group("/group")
	groupRoute.Use(middleware.AdminAuth())
	{
		groupRoute.GET("/", gatewayhttp.GetGroups)
	}

	prefillGroupRoute := apiRouter.Group("/prefill_group")
	prefillGroupRoute.Use(middleware.AdminAuth())
	{
		prefillGroupRoute.GET("/", GetPrefillGroups)
		prefillGroupRoute.POST("/", CreatePrefillGroup)
		prefillGroupRoute.PUT("/", UpdatePrefillGroup)
		prefillGroupRoute.DELETE("/:id", DeletePrefillGroup)
	}

	vendorRoute := apiRouter.Group("/vendors")
	vendorRoute.Use(middleware.AdminAuth())
	{
		vendorRoute.GET("/", GetAllVendors)
		vendorRoute.GET("/search", SearchVendors)
		vendorRoute.GET("/:id", GetVendorMeta)
		vendorRoute.POST("/", CreateVendorMeta)
		vendorRoute.PUT("/", UpdateVendorMeta)
		vendorRoute.DELETE("/:id", DeleteVendorMeta)
	}

}
