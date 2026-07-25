package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
	workflowhttp "github.com/sh2001sh/CodeGo-Api/internal/workflow/transport/http"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func RegisterGatewayRuntimeRoutes(router *gin.Engine) {
	router.Use(middleware.CORS())
	router.Use(middleware.DecompressRequestMiddleware())
	router.Use(middleware.BodyStorageCleanup())
	router.Use(middleware.StatsMiddleware())

	registerRelayModelRoutes(router)
	registerRelayPlaygroundRoutes(router)
	registerRelayCoreRoutes(router)
	registerRelayTaskRoutes(router)
}

func registerRelayModelRoutes(router *gin.Engine) {
	modelsRouter := router.Group("/v1/models")
	modelsRouter.Use(middleware.RouteTag("relay"))
	modelsRouter.Use(middleware.TokenAuth())
	{
		modelsRouter.GET("", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				gatewayhttp.ListModels(c, constant.ChannelTypeAnthropic)
			case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "":
				gatewayhttp.RetrieveModel(c, constant.ChannelTypeGemini)
			default:
				gatewayhttp.ListModels(c, constant.ChannelTypeOpenAI)
			}
		})

		modelsRouter.GET("/:model", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				gatewayhttp.RetrieveModel(c, constant.ChannelTypeAnthropic)
			default:
				gatewayhttp.RetrieveModel(c, constant.ChannelTypeOpenAI)
			}
		})
	}

	geminiRouter := router.Group("/v1beta/models")
	geminiRouter.Use(middleware.RouteTag("relay"))
	geminiRouter.Use(middleware.TokenAuth())
	{
		geminiRouter.GET("", func(c *gin.Context) {
			gatewayhttp.ListModels(c, constant.ChannelTypeGemini)
		})
	}

	geminiCompatibleRouter := router.Group("/v1beta/openai/models")
	geminiCompatibleRouter.Use(middleware.RouteTag("relay"))
	geminiCompatibleRouter.Use(middleware.TokenAuth())
	{
		geminiCompatibleRouter.GET("", func(c *gin.Context) {
			gatewayhttp.ListModels(c, constant.ChannelTypeOpenAI)
		})
	}
}

func registerRelayPlaygroundRoutes(router *gin.Engine) {
	playgroundRouter := router.Group("/pg")
	playgroundRouter.Use(middleware.RouteTag("relay"))
	playgroundRouter.Use(middleware.SystemPerformanceCheck())
	playgroundRouter.Use(middleware.UserAuth(), middleware.Distribute())
	{
		playgroundRouter.POST("/chat/completions", gatewayhttp.Playground)
		playgroundRouter.POST("/images/generations", gatewayhttp.PlaygroundImage)
		playgroundRouter.POST("/images/edits", gatewayhttp.PlaygroundImage)
	}
}

func registerRelayCoreRoutes(router *gin.Engine) {
	relayV1Router := router.Group("/v1")
	relayV1Router.Use(middleware.RouteTag("relay"))
	relayV1Router.Use(middleware.SystemPerformanceCheck())
	relayV1Router.Use(middleware.TokenAuth())
	relayV1Router.Use(middleware.ModelRequestRateLimit())
	{
		wsRouter := relayV1Router.Group("")
		wsRouter.Use(middleware.Distribute())
		wsRouter.GET("/realtime", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIRealtime))
	}
	{
		httpRouter := relayV1Router.Group("")
		httpRouter.Use(middleware.Distribute())

		httpRouter.POST("/messages", gatewayhttp.RelayWithFormat(types.RelayFormatClaude))
		httpRouter.POST("/completions", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAI))
		httpRouter.POST("/chat/completions", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAI))
		httpRouter.POST("/responses", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIResponses))
		httpRouter.POST("/responses/compact", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIResponsesCompaction))
		httpRouter.POST("/edits", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIImage))
		httpRouter.POST("/images/generations", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIImage))
		httpRouter.POST("/images/edits", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIImage))
		httpRouter.POST("/embeddings", gatewayhttp.RelayWithFormat(types.RelayFormatEmbedding))
		httpRouter.POST("/audio/transcriptions", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIAudio))
		httpRouter.POST("/audio/translations", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIAudio))
		httpRouter.POST("/audio/speech", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAIAudio))
		httpRouter.POST("/rerank", gatewayhttp.RelayWithFormat(types.RelayFormatRerank))
		httpRouter.POST("/engines/:model/embeddings", gatewayhttp.RelayWithFormat(types.RelayFormatGemini))
		httpRouter.POST("/models/*path", gatewayhttp.RelayWithFormat(types.RelayFormatGemini))
		httpRouter.POST("/moderations", gatewayhttp.RelayWithFormat(types.RelayFormatOpenAI))

		httpRouter.POST("/images/variations", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/files", gatewayhttp.RelayNotImplemented)
		httpRouter.POST("/files", gatewayhttp.RelayNotImplemented)
		httpRouter.DELETE("/files/:id", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/files/:id", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/files/:id/content", gatewayhttp.RelayNotImplemented)
		httpRouter.POST("/fine-tunes", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/fine-tunes", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id", gatewayhttp.RelayNotImplemented)
		httpRouter.POST("/fine-tunes/:id/cancel", gatewayhttp.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id/events", gatewayhttp.RelayNotImplemented)
		httpRouter.DELETE("/models/:model", gatewayhttp.RelayNotImplemented)
	}

	relayGeminiRouter := router.Group("/v1beta")
	relayGeminiRouter.Use(middleware.RouteTag("relay"))
	relayGeminiRouter.Use(middleware.SystemPerformanceCheck())
	relayGeminiRouter.Use(middleware.TokenAuth())
	relayGeminiRouter.Use(middleware.ModelRequestRateLimit())
	relayGeminiRouter.Use(middleware.Distribute())
	{
		relayGeminiRouter.POST("/models/*path", gatewayhttp.RelayWithFormat(types.RelayFormatGemini))
	}
}

func registerRelayTaskRoutes(router *gin.Engine) {
	relaySunoRouter := router.Group("/suno")
	relaySunoRouter.Use(middleware.RouteTag("relay"))
	relaySunoRouter.Use(middleware.SystemPerformanceCheck())
	relaySunoRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		relaySunoRouter.POST("/submit/:action", workflowhttp.SubmitRelayTask)
		relaySunoRouter.POST("/fetch", workflowhttp.FetchRelayTask)
		relaySunoRouter.GET("/fetch/:id", workflowhttp.FetchRelayTask)
	}

	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		videoProxyRouter.GET("/videos/:task_id/content", workflowhttp.VideoProxy)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", workflowhttp.SubmitRelayTask)
		videoV1Router.GET("/video/generations/:task_id", workflowhttp.FetchRelayTask)
		videoV1Router.POST("/videos/:video_id/remix", workflowhttp.SubmitRelayTask)
		videoV1Router.POST("/videos", workflowhttp.SubmitRelayTask)
		videoV1Router.GET("/videos/:task_id", workflowhttp.FetchRelayTask)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", workflowhttp.SubmitRelayTask)
		klingV1Router.POST("/videos/image2video", workflowhttp.SubmitRelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", workflowhttp.FetchRelayTask)
		klingV1Router.GET("/videos/image2video/:task_id", workflowhttp.FetchRelayTask)
	}

	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		jimengOfficialGroup.POST("/", workflowhttp.SubmitRelayTask)
	}
}
