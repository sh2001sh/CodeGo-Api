package app

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	"gorm.io/gorm"
)

func RecordLog(userID int, logType int, content string) {
	projection.RecordLog(userID, logType, content)
}

func RecordLogTx(tx *gorm.DB, userID int, logType int, content string) error {
	return projection.RecordLogTx(tx, userID, logType, content)
}

func RecordLogWithAdminInfo(userID int, logType int, content string, adminInfo map[string]interface{}) {
	projection.RecordLogWithAdminInfo(userID, logType, content, adminInfo)
}

func RecordTopupLog(userID int, content string, callerIP string, paymentMethod string, callbackPaymentMethod string) {
	projection.RecordTopupLog(userID, content, callerIP, paymentMethod, callbackPaymentMethod)
}

func RecordErrorLog(c *gin.Context, userID int, channelID int, modelName string, tokenName string, content string, tokenID int, useTimeSeconds int, isStream bool, group string, other map[string]interface{}) {
	projection.RecordErrorLog(c, userID, channelID, modelName, tokenName, content, tokenID, useTimeSeconds, isStream, group, other)
}

func RecordConsumeLog(c *gin.Context, userID int, params auditschema.RecordConsumeLogParams) {
	projection.RecordConsumeLog(c, userID, params)
}

func RecordTaskBillingLog(params auditschema.RecordTaskBillingLogParams) {
	projection.RecordTaskBillingLog(params)
}
