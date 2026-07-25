package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterGatewayRoutes(apiRouter *gin.RouterGroup) {
	routePoolRoute := apiRouter.Group("/route-pools")
	routePoolRoute.Use(middleware.RootAuth())
	{
		routePoolRoute.GET("/groups", ListRoutePoolGroups)
		routePoolRoute.PUT("/groups", SaveRoutePoolGroup)
		routePoolRoute.GET("/", ListRoutePools)
		routePoolRoute.POST("/", SaveRoutePool)
		routePoolRoute.PUT("/", SaveRoutePool)
		routePoolRoute.DELETE("/:id", DeleteRoutePool)
		routePoolRoute.GET("/:id/metrics", GetRoutePoolMetrics)
	}
	channelRoute := apiRouter.Group("/channel")
	channelRoute.Use(middleware.AdminAuth())
	{
		channelRoute.GET("/", GetAllChannels)
		channelRoute.GET("/search", SearchChannels)
		channelRoute.GET("/models", ChannelListModels)
		channelRoute.GET("/models_enabled", EnabledListModels)
		channelRoute.GET("/:id", GetChannel)
		channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), GetChannelKey)
		channelRoute.GET("/test", TestAllChannels)
		channelRoute.GET("/test/:id", TestChannel)
		channelRoute.GET("/update_balance", UpdateAllChannelsBalance)
		channelRoute.GET("/update_balance/:id", UpdateChannelBalance)
		channelRoute.POST("/", AddChannel)
		channelRoute.PUT("/", UpdateChannel)
		channelRoute.DELETE("/disabled", DeleteDisabledChannels)
		channelRoute.POST("/tag/disabled", DisableTagChannels)
		channelRoute.POST("/tag/enabled", EnableTagChannels)
		channelRoute.PUT("/tag", EditTagChannels)
		channelRoute.DELETE("/:id", DeleteChannel)
		channelRoute.POST("/batch", DeleteChannelBatch)
		channelRoute.POST("/fix", FixChannelsAbilities)
		channelRoute.GET("/fetch_models/:id", FetchUpstreamModels)
		channelRoute.POST("/fetch_models", middleware.RootAuth(), FetchModels)
		channelRoute.POST("/codex/oauth/start", StartCodexOAuth)
		channelRoute.POST("/codex/oauth/complete", CompleteCodexOAuth)
		channelRoute.POST("/:id/codex/oauth/start", StartCodexOAuthForChannel)
		channelRoute.POST("/:id/codex/oauth/complete", CompleteCodexOAuthForChannel)
		channelRoute.POST("/:id/codex/refresh", RefreshCodexChannelCredential)
		channelRoute.GET("/:id/codex/usage", GetCodexChannelUsage)
		channelRoute.POST("/ollama/pull", PullOllamaModel)
		channelRoute.POST("/ollama/pull/stream", PullOllamaModelStream)
		channelRoute.DELETE("/ollama/delete", DeleteOllamaModel)
		channelRoute.GET("/ollama/version/:id", OllamaVersion)
		channelRoute.POST("/batch/tag", BatchSetChannelTag)
		channelRoute.GET("/tag/models", GetTagModels)
		channelRoute.POST("/copy/:id", CopyChannel)
		channelRoute.POST("/multi_key/manage", ManageMultiKeys)
		channelRoute.POST("/upstream_updates/apply", ApplyChannelUpstreamModelUpdates)
		channelRoute.POST("/upstream_updates/apply_all", ApplyAllChannelUpstreamModelUpdates)
		channelRoute.POST("/upstream_updates/detect", DetectChannelUpstreamModelUpdates)
		channelRoute.POST("/upstream_updates/detect_all", DetectAllChannelUpstreamModelUpdates)
	}

	modelsRoute := apiRouter.Group("/models")
	modelsRoute.Use(middleware.AdminAuth())
	{
		modelsRoute.GET("/sync_upstream/preview", SyncUpstreamPreview)
		modelsRoute.POST("/sync_upstream", SyncUpstreamModels)
		modelsRoute.GET("/missing", GetMissingModels)
		modelsRoute.GET("/", GetAllModelsMeta)
		modelsRoute.GET("/search", SearchModelsMeta)
		modelsRoute.GET("/:id", GetModelMeta)
		modelsRoute.POST("/", CreateModelMeta)
		modelsRoute.PUT("/", UpdateModelMeta)
		modelsRoute.DELETE("/:id", DeleteModelMeta)
	}
}
