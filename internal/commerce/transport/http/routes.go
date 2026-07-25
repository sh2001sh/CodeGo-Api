package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterCommerceRoutes(apiRouter *gin.RouterGroup, anonymousRequestBodyLimit gin.HandlerFunc) {
	subscriptionRoute := apiRouter.Group("/subscription")
	subscriptionRoute.Use(middleware.UserAuth())
	{
		subscriptionRoute.GET("/plans", getSubscriptionPlans)
		subscriptionRoute.GET("/self", getSubscriptionSelf)
		subscriptionRoute.GET("/orders/:trade_no", getSubscriptionOrderStatus)
		subscriptionRoute.POST("/orders/:trade_no/cancel", middleware.CriticalRateLimit(), cancelSubscriptionOrder)
		subscriptionRoute.PUT("/self/preference", updateSubscriptionPreference)
		subscriptionRoute.POST("/fuel/quote", quoteSubscriptionFuel)
		subscriptionRoute.POST("/fuel/purchase", middleware.CriticalRateLimit(), purchaseSubscriptionFuel)
		subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), RequestSubscriptionStripePay)
		subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), RequestSubscriptionCreemPay)
	}

	subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
	subscriptionAdminRoute.Use(middleware.AdminAuth())
	{
		subscriptionAdminRoute.GET("/plans", listAdminSubscriptionPlans)
		subscriptionAdminRoute.POST("/plans", createAdminSubscriptionPlan)
		subscriptionAdminRoute.PUT("/plans/:id", updateAdminSubscriptionPlan)
		subscriptionAdminRoute.PATCH("/plans/:id", updateAdminSubscriptionPlanStatus)
		subscriptionAdminRoute.DELETE("/plans/:id", deleteAdminSubscriptionPlan)
		subscriptionAdminRoute.POST("/bind", bindAdminSubscription)
		subscriptionAdminRoute.GET("/users/:id/subscriptions", listAdminUserSubscriptions)
		subscriptionAdminRoute.POST("/users/:id/subscriptions", createAdminUserSubscription)
		subscriptionAdminRoute.PUT("/user_subscriptions/:id", updateAdminUserSubscription)
		subscriptionAdminRoute.POST("/user_subscriptions/:id/reset", resetAdminUserSubscriptionQuota)
		subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", invalidateAdminUserSubscription)
		subscriptionAdminRoute.DELETE("/user_subscriptions/:id", deleteAdminUserSubscription)
	}

	packagesRoute := apiRouter.Group("/packages")
	packagesRoute.Use(middleware.UserAuth())
	{
		packagesRoute.GET("/public", getPublicPackages)
		packagesRoute.GET("/my-subscription", getSubscriptionSelf)
		packagesRoute.POST("/purchase", middleware.CriticalRateLimit(), PurchasePackage)
		packagesRoute.POST("/upgrade", middleware.CriticalRateLimit(), UpgradePackage)
		packagesRoute.POST("/renew", middleware.CriticalRateLimit(), RenewPackage)
	}

}
