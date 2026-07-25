package http

import (
	adminopshttp "github.com/sh2001sh/CodeGo-Api/internal/adminops/transport/http"
	audithttp "github.com/sh2001sh/CodeGo-Api/internal/audit/transport/http"
	billinghttp "github.com/sh2001sh/CodeGo-Api/internal/billing/transport/http"
	commercehttp "github.com/sh2001sh/CodeGo-Api/internal/commerce/transport/http"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	identityhttp "github.com/sh2001sh/CodeGo-Api/internal/identity/transport/http"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
	workflowhttp "github.com/sh2001sh/CodeGo-Api/internal/workflow/transport/http"

	_ "github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RegisterControlRuntimeRoutes(router *gin.Engine, assets ThemeAssets) {
	registerControlAPIRoutes(router)
	registerDashboardCompatibilityRoutes(router)
	registerControlWebRoutes(router, assets)
}

func registerControlAPIRoutes(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup())
	apiRouter.Use(middleware.GlobalAPIRateLimitExceptReadPaths())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		RegisterPlatformRoutes(apiRouter, anonymousRequestBodyLimit)
		apiRouter.GET("/models", middleware.UserAuth(), gatewayhttp.DashboardListModels)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), gatewayhttp.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", gatewayhttp.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", gatewayhttp.GetPerfMetrics)
		}
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), identityhttp.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, identityhttp.BindEmail)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), identityhttp.HandleWeChatOAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, identityhttp.BindWeChatOAuth)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), identityhttp.HandleTelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), identityhttp.BindTelegramOAuth)
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), identityhttp.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), gatewayhttp.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, commercehttp.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, commercehttp.CreemWebhook)

		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), identityhttp.UniversalVerify)

		identityhttp.RegisterUserRoutes(apiRouter, anonymousRequestBodyLimit)
		commercehttp.RegisterCommerceRoutes(apiRouter, anonymousRequestBodyLimit)
		identityhttp.RegisterTokenRoutes(apiRouter)
		gatewayhttp.RegisterGatewayRoutes(apiRouter)
		adminopshttp.RegisterAdminOpsRoutes(apiRouter)
		audithttp.RegisterAuditRoutes(apiRouter)
		workflowhttp.RegisterWorkflowRoutes(apiRouter)
		billinghttp.RegisterBillingRoutes(apiRouter)
	}
}

func registerDashboardCompatibilityRoutes(router *gin.Engine) {
	apiRouter := router.Group("/")
	apiRouter.Use(middleware.RouteTag("old_api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimitExceptReadPaths())
	apiRouter.Use(middleware.CORS())
	apiRouter.Use(middleware.TokenAuth())
	{
		apiRouter.GET("/dashboard/billing/subscription", gatewayhttp.GetOpenAIProtocolSubscription)
		apiRouter.GET("/v1/dashboard/billing/subscription", gatewayhttp.GetOpenAIProtocolSubscription)
		apiRouter.GET("/dashboard/billing/usage", gatewayhttp.GetOpenAIProtocolUsage)
		apiRouter.GET("/v1/dashboard/billing/usage", gatewayhttp.GetOpenAIProtocolUsage)
	}
}
