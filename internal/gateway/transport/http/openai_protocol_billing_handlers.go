package http

import (
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func GetOpenAIProtocolSubscription(c *gin.Context) {
	var (
		remainQuota int
		usedQuota   int
		expiredTime int64
		token       *billingapp.TokenSnapshot
		err         error
	)

	if platformconfig.DisplayTokenStatEnabled {
		tokenID := c.GetInt("token_id")
		token, err = billingapp.GetTokenByID(tokenID)
		if err == nil {
			expiredTime = token.ExpiredTime
			remainQuota = token.RemainQuota
			usedQuota = token.UsedQuota
		}
	} else {
		userID := c.GetInt("id")
		remainQuota, err = billingapp.GetUserWalletQuota(userID)
		if err == nil {
			usedQuota, err = billingapp.GetUserUsedQuota(userID)
		}
	}

	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"error": types.OpenAIError{
				Message: err.Error(),
				Type:    "upstream_error",
			},
		})
		return
	}

	amount := quotaToOpenAIUSD(remainQuota + usedQuota)
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}

	c.JSON(stdhttp.StatusOK, openAIProtocolSubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	})
}

func GetOpenAIProtocolUsage(c *gin.Context) {
	var (
		quota int
		err   error
	)

	if platformconfig.DisplayTokenStatEnabled {
		tokenID := c.GetInt("token_id")
		var token *billingapp.TokenSnapshot
		token, err = billingapp.GetTokenByID(tokenID)
		if err == nil {
			quota = token.UsedQuota
		}
	} else {
		userID := c.GetInt("id")
		quota, err = billingapp.GetUserUsedQuota(userID)
	}

	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"error": types.OpenAIError{
				Message: err.Error(),
				Type:    "codego_api_error",
			},
		})
		return
	}

	c.JSON(stdhttp.StatusOK, openAIProtocolUsageResponse{
		Object:     "list",
		TotalUsage: quotaToOpenAIUsage(quota),
	})
}
