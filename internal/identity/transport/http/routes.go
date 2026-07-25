package http

import (
	"github.com/gin-gonic/gin"
	commercehttp "github.com/sh2001sh/CodeGo-Api/internal/commerce/transport/http"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterUserRoutes(apiRouter *gin.RouterGroup, anonymousRequestBodyLimit gin.HandlerFunc) {
	apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), SendEmailVerification)
	apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), SendPasswordResetEmail)
	apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, ResetPassword)

	userRoute := apiRouter.Group("/user")
	{
		userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), Register)
		userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), Login)
		userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, Verify2FALogin)
		userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, PasskeyLoginBegin)
		userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, PasskeyLoginFinish)
		userRoute.GET("/logout", Logout)
		userRoute.GET("/groups", gatewayhttp.GetUserGroups)

		selfRoute := userRoute.Group("/")
		selfRoute.Use(middleware.UserAuth())
		{
			selfRoute.GET("/self/groups", gatewayhttp.GetUserGroups)
			selfRoute.GET("/self/group-status", gatewayhttp.GetUserGroupStatus)
			selfRoute.GET("/self", GetUserSelf)
			selfRoute.GET("/models", GetUserModels)
			selfRoute.GET("/image-workspace/models", GetImageWorkspaceModels)
			selfRoute.GET("/image-workspace/items", GetImageWorkspaceItems)
			selfRoute.GET("/image-workspace/items/:id/content", GetImageWorkspaceItemContent)
			selfRoute.PUT("/self", UpdateSelf)
			selfRoute.DELETE("/self", DeleteSelf)
			selfRoute.GET("/token", GenerateAccessToken)
			selfRoute.GET("/token/status", GetTokenStatus)
			selfRoute.GET("/passkey", PasskeyStatus)
			selfRoute.POST("/passkey/register/begin", PasskeyRegisterBegin)
			selfRoute.POST("/passkey/register/finish", PasskeyRegisterFinish)
			selfRoute.POST("/passkey/verify/begin", PasskeyVerifyBegin)
			selfRoute.POST("/passkey/verify/finish", PasskeyVerifyFinish)
			selfRoute.DELETE("/passkey", PasskeyDelete)
			selfRoute.GET("/topup/info", commercehttp.GetTopUpInfo)
			selfRoute.GET("/topup/self", commercehttp.GetUserTopUps)
			selfRoute.POST("/topup", middleware.CriticalRateLimit(), commercehttp.RedeemTopUpCode)
			selfRoute.POST("/amount", commercehttp.RequestAmount)
			selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), commercehttp.RequestStripePay)
			selfRoute.POST("/stripe/amount", commercehttp.RequestStripeAmount)
			selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), commercehttp.RequestCreemPay)
			selfRoute.PUT("/setting", UpdateUserSetting)
			selfRoute.GET("/2fa/status", GetTwoFAStatus)
			selfRoute.POST("/2fa/setup", SetupTwoFA)
			selfRoute.POST("/2fa/enable", EnableTwoFA)
			selfRoute.POST("/2fa/disable", DisableTwoFA)
			selfRoute.POST("/2fa/backup_codes", RegenerateBackupCodes)
			selfRoute.GET("/oauth/bindings", GetUserOAuthBindings)
			selfRoute.DELETE("/oauth/bindings/:provider_id", UnbindCustomOAuth)
		}

		adminRoute := userRoute.Group("/")
		adminRoute.Use(middleware.AdminAuth())
		{
			adminRoute.GET("/", GetAllUsers)
			adminRoute.GET("/topup", commercehttp.GetAllTopUps)
			adminRoute.POST("/topup/complete", commercehttp.AdminCompleteTopUp)
			adminRoute.GET("/search", SearchUsers)
			adminRoute.GET("/:id/oauth/bindings", GetUserOAuthBindingsByAdmin)
			adminRoute.DELETE("/:id/oauth/bindings/:provider_id", UnbindCustomOAuthByAdmin)
			adminRoute.DELETE("/:id/bindings/:binding_type", AdminClearUserBinding)
			adminRoute.GET("/:id", GetUser)
			adminRoute.POST("/", CreateUser)
			adminRoute.POST("/manage", ManageUser)
			adminRoute.PUT("/", UpdateUser)
			adminRoute.DELETE("/:id", DeleteUser)
			adminRoute.DELETE("/:id/reset_passkey", AdminResetPasskey)
			adminRoute.GET("/2fa/stats", Admin2FAStats)
			adminRoute.DELETE("/:id/2fa", AdminDisable2FA)
		}
	}
}

func RegisterTokenRoutes(apiRouter *gin.RouterGroup) {
	tokenRoute := apiRouter.Group("/token")
	tokenRoute.Use(middleware.UserAuth())
	{
		tokenRoute.GET("/", GetAllTokens)
		tokenRoute.GET("/search", middleware.SearchRateLimit(), SearchTokens)
		tokenRoute.GET("/:id", GetToken)
		tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), GetTokenKey)
		tokenRoute.POST("/", AddToken)
		tokenRoute.PUT("/", UpdateToken)
		tokenRoute.DELETE("/:id", DeleteToken)
		tokenRoute.POST("/batch", DeleteTokenBatch)
		tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), GetTokenKeysBatch)
	}

	usageRoute := apiRouter.Group("/usage")
	usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
	{
		tokenUsageRoute := usageRoute.Group("/token")
		tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
		{
			tokenUsageRoute.GET("/", GetTokenUsage)
		}
	}
}
