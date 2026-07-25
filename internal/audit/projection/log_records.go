package projection

import (
	"fmt"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"gorm.io/gorm"
	"os"
)

func RecordLog(userID int, logType int, content string) {
	if err := RecordLogTx(nil, userID, logType, content); err != nil {
		platformobservability.SysLog("failed to record log: " + err.Error())
	}
}

func RecordLogTx(tx *gorm.DB, userID int, logType int, content string) error {
	if logType == auditschema.LogTypeConsume && !platformconfig.LogConsumeEnabled {
		return nil
	}
	targetDB := platformdb.LogDB
	if targetDB == nil {
		targetDB = platformdb.DB
	}
	if tx != nil && os.Getenv("LOG_SQL_DSN") == "" {
		targetDB = tx
	}
	username := ""
	if userID > 0 {
		_ = targetDB.Model(&identityschema.User{}).Where("id = ?", userID).Select("username").Find(&username).Error
	}
	logRow := &auditschema.Log{
		UserId:    userID,
		Username:  username,
		CreatedAt: platformruntime.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	return targetDB.Create(logRow).Error
}

func RecordLogWithAdminInfo(userID int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == auditschema.LogTypeConsume && !platformconfig.LogConsumeEnabled {
		return
	}
	username, _ := identitystore.LoadUsernameByID(userID, false)
	logRow := &auditschema.Log{
		UserId:    userID,
		Username:  username,
		CreatedAt: platformruntime.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		logRow.Other = platformtext.MapToJsonStr(map[string]interface{}{"admin_info": adminInfo})
	}
	if err := platformdb.LogDB.Create(logRow).Error; err != nil {
		platformobservability.SysLog("failed to record log: " + err.Error())
	}
}

func RecordTopupLog(userID int, content string, callerIP string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := identitystore.LoadUsernameByID(userID, false)
	adminInfo := map[string]interface{}{
		"server_ip":               platformruntime.GetIP(),
		"node_name":               platformconfig.NodeName,
		"caller_ip":               callerIP,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 platformconfig.Version,
	}
	logRow := &auditschema.Log{
		UserId:    userID,
		Username:  username,
		CreatedAt: platformruntime.GetTimestamp(),
		Type:      auditschema.LogTypeTopup,
		Content:   content,
		Ip:        callerIP,
		Other:     platformtext.MapToJsonStr(map[string]interface{}{"admin_info": adminInfo}),
	}
	if err := platformdb.LogDB.Create(logRow).Error; err != nil {
		platformobservability.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userID int, channelID int, modelName string, tokenName string, content string, tokenID int, useTimeSeconds int, isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userID, channelID, modelName, tokenName, content))
	username := c.GetString("username")
	requestID := c.GetString(constant.RequestIdKey)
	upstreamRequestID := c.GetString(constant.UpstreamRequestIdKey)
	needRecordIP := false
	if settingMap, err := identitystore.LoadUserSetting(userID, false); err == nil {
		needRecordIP = settingMap.RecordIpLog
	}
	logRow := &auditschema.Log{
		UserId:            userID,
		Username:          username,
		CreatedAt:         platformruntime.GetTimestamp(),
		Type:              auditschema.LogTypeError,
		Content:           content,
		TokenName:         tokenName,
		ModelName:         modelName,
		ChannelId:         channelID,
		TokenId:           tokenID,
		UseTime:           useTimeSeconds,
		IsStream:          isStream,
		Group:             group,
		RequestId:         requestID,
		UpstreamRequestId: upstreamRequestID,
		Other:             platformtext.MapToJsonStr(other),
	}
	if needRecordIP {
		logRow.Ip = c.ClientIP()
	}
	if err := platformdb.LogDB.Create(logRow).Error; err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

func RecordConsumeLog(c *gin.Context, userID int, params auditschema.RecordConsumeLogParams) {
	if !platformconfig.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userID, platformtext.GetJsonString(params)))
	username := c.GetString("username")
	requestID := c.GetString(constant.RequestIdKey)
	upstreamRequestID := c.GetString(constant.UpstreamRequestIdKey)
	needRecordIP := false
	if settingMap, err := identitystore.LoadUserSetting(userID, false); err == nil {
		needRecordIP = settingMap.RecordIpLog
	}

	logRow := &auditschema.Log{
		UserId:            userID,
		Username:          username,
		CreatedAt:         platformruntime.GetTimestamp(),
		Type:              auditschema.LogTypeConsume,
		Content:           params.Content,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		TokenName:         params.TokenName,
		ModelName:         params.ModelName,
		Quota:             params.Quota,
		ChannelId:         params.ChannelId,
		TokenId:           params.TokenId,
		UseTime:           params.UseTimeSeconds,
		IsStream:          params.IsStream,
		Group:             params.Group,
		RequestId:         requestID,
		UpstreamRequestId: upstreamRequestID,
		Other:             platformtext.MapToJsonStr(params.Other),
	}
	if needRecordIP {
		logRow.Ip = c.ClientIP()
	}
	err := platformdb.LogDB.Create(logRow).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	if platformconfig.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userID, username, params.ModelName, params.Quota, platformruntime.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

func RecordTaskBillingLog(params auditschema.RecordTaskBillingLogParams) {
	if params.LogType == auditschema.LogTypeConsume && !platformconfig.LogConsumeEnabled {
		return
	}
	username, _ := identitystore.LoadUsernameByID(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if name, err := getAuditTokenNameByID(params.TokenId); err == nil {
			tokenName = name
		}
	}
	logRow := &auditschema.Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: platformruntime.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     platformtext.MapToJsonStr(params.Other),
	}
	if err := platformdb.LogDB.Create(logRow).Error; err != nil {
		platformobservability.SysLog("failed to record task billing log: " + err.Error())
	}
}
