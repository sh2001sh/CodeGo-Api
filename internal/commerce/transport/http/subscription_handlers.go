package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"strings"

	"github.com/gin-gonic/gin"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
)

func quoteSubscriptionFuel(c *gin.Context) {
	var req commerceapp.SubscriptionFuelQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	payload, err := commerceapp.QuoteSubscriptionFuel(c.GetInt("id"), req)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func purchaseSubscriptionFuel(c *gin.Context) {
	var req commerceapp.SubscriptionFuelPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	if req.PaymentMethod == commerceschema.PaymentMethodStripe {
		payload, err := commerceapp.CreateSubscriptionFuelStripePayment(c.GetInt("id"), req)
		if err != nil {
			httpapi.ApiError(c, err)
			return
		}
		httpapi.ApiSuccess(c, payload)
		return
	}
	httpapi.ApiErrorMsg(c, "unsupported payment method")
}

func getSubscriptionPlans(c *gin.Context) {
	payload, err := commerceapp.ListSubscriptionPlans(c.GetInt("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func getPublicPackages(c *gin.Context) {
	getSubscriptionPlans(c)
}

func getSubscriptionOrderStatus(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	if tradeNo == "" {
		httpapi.ApiErrorMsg(c, "invalid trade no")
		return
	}
	payload, err := commerceapp.BuildSubscriptionOrderStatusPayload(c.GetInt("id"), tradeNo)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func cancelSubscriptionOrder(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	if tradeNo == "" {
		httpapi.ApiErrorMsg(c, "invalid trade no")
		return
	}
	if err := commerceapp.CancelPendingSubscriptionOrder(c.GetInt("id"), tradeNo); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func getSubscriptionSelf(c *gin.Context) {
	payload, err := commerceapp.BuildSubscriptionSelfPayload(c.GetInt("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}

func updateSubscriptionPreference(c *gin.Context) {
	var req commerceapp.UpdateSubscriptionPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}
	payload, err := commerceapp.UpdateSubscriptionPreference(c.GetInt("id"), req)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, payload)
}
