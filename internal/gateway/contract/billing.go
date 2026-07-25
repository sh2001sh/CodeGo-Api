package contract

import "github.com/gin-gonic/gin"

type BillingSettler interface {
	Settle(actualQuota int) error
	Refund(c *gin.Context)
	NeedsRefund() bool
	GetPreConsumedQuota() int
	Reserve(targetQuota int) error
}
