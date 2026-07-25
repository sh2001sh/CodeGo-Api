package projection

import (
	"errors"
	"fmt"
	auditdomain "github.com/sh2001sh/CodeGo-Api/internal/audit/domain"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

func LogQuotaData(userID int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	createdAt -= createdAt % 3600
	if err := persistQuotaData(userID, username, modelName, quota, createdAt, tokenUsed); err != nil {
		platformobservability.SysLog(fmt.Sprintf("save quota data failed: %v", err))
	}
}

func persistQuotaData(userID int, username string, modelName string, quota int, createdAt int64, tokenUsed int) error {
	quotaData := &auditdomain.QuotaData{}
	err := platformdb.DB.Table("quota_data").
		Where("user_id = ? and username = ? and model_name = ? and created_at = ?", userID, username, modelName, createdAt).
		First(quotaData).
		Error
	if err == nil {
		return increaseQuotaData(userID, username, modelName, 1, quota, createdAt, tokenUsed)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return platformdb.DB.Table("quota_data").Create(&auditdomain.QuotaData{
		UserID:    userID,
		Username:  username,
		ModelName: modelName,
		CreatedAt: createdAt,
		Count:     1,
		Quota:     quota,
		TokenUsed: tokenUsed,
	}).Error
}

func increaseQuotaData(userID int, username string, modelName string, count int, quota int, createdAt int64, tokenUsed int) error {
	err := platformdb.DB.Table("quota_data").
		Where("user_id = ? and username = ? and model_name = ? and created_at = ?", userID, username, modelName, createdAt).
		Updates(map[string]interface{}{
			"count":      gorm.Expr("count + ?", count),
			"quota":      gorm.Expr("quota + ?", quota),
			"token_used": gorm.Expr("token_used + ?", tokenUsed),
		}).Error
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
	return err
}

func GetQuotaDataByUserID(userID int, startTime int64, endTime int64) ([]*auditdomain.QuotaData, error) {
	var rows []*auditdomain.QuotaData
	err := platformdb.DB.Table("quota_data").
		Where("user_id = ? and created_at >= ? and created_at <= ?", userID, startTime, endTime).
		Find(&rows).
		Error
	return rows, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) ([]*auditdomain.QuotaData, error) {
	var rows []*auditdomain.QuotaData
	err := platformdb.DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&rows).
		Error
	return rows, err
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) ([]*auditdomain.QuotaData, error) {
	if username != "" {
		var rows []*auditdomain.QuotaData
		err := platformdb.DB.Table("quota_data").
			Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).
			Find(&rows).
			Error
		return rows, err
	}
	var rows []*auditdomain.QuotaData
	err := platformdb.DB.Table("quota_data").
		Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, created_at").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("model_name, created_at").
		Find(&rows).
		Error
	return rows, err
}
