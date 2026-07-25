package http

import (
	"github.com/gin-gonic/gin"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	stdhttp "net/http"
)

func RequestStripeAmount(c *gin.Context) {
	var req commerceapp.StripePayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": "鍙傛暟閿欒"})
		return
	}
	amount, err := commerceapp.QuoteStripeTopUpAmount(c.GetInt("id"), req)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"message": "success", "data": amount})
}

func RequestStripePay(c *gin.Context) {
	var req commerceapp.StripePayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": "鍙傛暟閿欒"})
		return
	}
	payload, err := commerceapp.CreateStripeTopUp(c.Request.Context(), c.GetInt("id"), req)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"message": "success", "data": payload})
}

func StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !commerceapp.IsStripeTopUpEnabled() {
		logger.LogWarn(ctx, "Stripe webhook 琚嫆缁?reason=webhook_disabled")
		c.AbortWithStatus(stdhttp.StatusForbidden)
		return
	}

	storage, err := platformhttpx.GetBodyStorage(c)
	if err != nil {
		c.AbortWithStatus(stdhttp.StatusServiceUnavailable)
		return
	}
	payload, err := storage.Bytes()
	if err != nil {
		c.AbortWithStatus(stdhttp.StatusServiceUnavailable)
		return
	}
	signature := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, signature, commercestore.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		c.AbortWithStatus(stdhttp.StatusBadRequest)
		return
	}

	referenceID := event.GetObjectValue("client_reference_id")
	customerID := event.GetObjectValue("customer")
	clientIP := c.ClientIP()
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		if event.GetObjectValue("status") == "complete" && event.GetObjectValue("payment_status") == "paid" {
			_ = commerceapp.HandleStripeWebhookFulfillment(ctx, referenceID, customerID, map[string]any{
				"customer":     customerID,
				"amount_total": event.GetObjectValue("amount_total"),
				"currency":     event.GetObjectValue("currency"),
				"event_type":   string(event.Type),
			}, clientIP)
		}
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		_ = commerceapp.HandleStripeWebhookFulfillment(ctx, referenceID, customerID, map[string]any{
			"customer":     customerID,
			"amount_total": event.GetObjectValue("amount_total"),
			"currency":     event.GetObjectValue("currency"),
			"event_type":   string(event.Type),
		}, clientIP)
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		_ = commerceapp.MarkStripeTopUpFailed(ctx, referenceID, clientIP)
	case stripe.EventTypeCheckoutSessionExpired:
		if event.GetObjectValue("status") == "expired" {
			_ = commerceapp.ExpireStripeOrder(ctx, referenceID)
		}
	}
	c.Status(stdhttp.StatusOK)
}
func RequestCreemPay(c *gin.Context) {
	var req commerceapp.CreemPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": "鍙傛暟閿欒"})
		return
	}
	payload, err := commerceapp.CreateCreemTopUp(c.Request.Context(), c.GetInt("id"), req)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"message": "success", "data": payload})
}

func CreemWebhook(c *gin.Context) {
	if !commerceapp.IsCreemWebhookEnabled() {
		c.AbortWithStatus(stdhttp.StatusForbidden)
		return
	}
	storage, err := platformhttpx.GetBodyStorage(c)
	if err != nil {
		c.AbortWithStatus(stdhttp.StatusBadRequest)
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		c.AbortWithStatus(stdhttp.StatusBadRequest)
		return
	}
	signature := c.GetHeader(commerceapp.CreemSignatureHeader)
	if signature == "" || !commerceapp.VerifyCreemSignature(string(body), signature, commercestore.CreemWebhookSecret) {
		c.AbortWithStatus(stdhttp.StatusUnauthorized)
		return
	}

	var event commerceapp.CreemWebhookEvent
	if err := platformencoding.Unmarshal(body, &event); err != nil {
		c.AbortWithStatus(stdhttp.StatusBadRequest)
		return
	}
	if event.EventType == "checkout.completed" {
		if err := commerceapp.HandleCreemCheckoutCompleted(c.Request.Context(), &event, c.ClientIP()); err != nil {
			c.AbortWithStatus(stdhttp.StatusInternalServerError)
			return
		}
	}
	c.Status(stdhttp.StatusOK)
}
