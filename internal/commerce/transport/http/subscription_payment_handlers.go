package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

func RequestSubscriptionStripePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req commerceapp.SubscriptionStripePayRequest
	if err := platformhttpx.UnmarshalBodyReusable(c, &req); err != nil || req.PlanID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}

	payload, err := commerceapp.CreateSubscriptionStripePayment(c.GetInt("id"), req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    payload,
	})
}

func RequestSubscriptionCreemPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req commerceapp.SubscriptionCreemPayRequest
	if err := platformhttpx.UnmarshalBodyReusable(c, &req); err != nil || req.PlanID <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "invalid request"})
		return
	}

	payload, err := commerceapp.CreateSubscriptionCreemPayment(c.GetInt("id"), req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    payload,
	})
}

func PurchasePackage(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req commerceapp.SubscriptionPurchaseFields
	if err := platformhttpx.UnmarshalBodyReusable(c, &req); err != nil || req.PlanID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid request")
		return
	}

	switch strings.ToLower(strings.TrimSpace(req.PaymentMethod)) {
	case commerceschema.PaymentMethodStripe:
		RequestSubscriptionStripePay(c)
	case commerceschema.PaymentMethodCreem:
		RequestSubscriptionCreemPay(c)
	default:
		httpapi.ApiErrorMsg(c, "unsupported payment method")
	}
}

func UpgradePackage(c *gin.Context) {
	PurchasePackage(c)
}

func RenewPackage(c *gin.Context) {
	PurchasePackage(c)
}

func requirePaymentCompliance(c *gin.Context) bool {
	if !commerceapp.IsPaymentComplianceConfirmed() {
		httpapi.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return false
	}
	return true
}
