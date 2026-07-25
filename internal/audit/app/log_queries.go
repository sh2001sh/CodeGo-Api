package app

import (
	"context"
	"errors"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"

	auditdomain "github.com/sh2001sh/CodeGo-Api/internal/audit/domain"
	"github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
)

const maxUserQuotaRangeSeconds int64 = 30 * 24 * 60 * 60

type LogListQuery = auditdomain.LogListQuery

func ListAdminLogs(query LogListQuery) ([]*auditschema.Log, int64, error) {
	return projection.ListAdminLogs(query)
}

func ListUserLogs(userID int, query LogListQuery) ([]*auditschema.Log, int64, error) {
	return projection.ListUserLogs(userID, query)
}

func GetLogsByTokenID(tokenID int) ([]*auditschema.Log, error) {
	return projection.GetLogByTokenID(tokenID)
}

func GetAdminLogStats(query LogListQuery) (auditschema.Stat, error) {
	return projection.SumUsedQuota(query)
}

func GetUserLogStats(username string, query LogListQuery) (auditschema.Stat, error) {
	query.Username = username
	return projection.SumUsedQuota(query)
}

func DeleteHistoryLogs(ctx context.Context, targetTimestamp int64) (int64, error) {
	if targetTimestamp == 0 {
		return 0, errors.New("target timestamp is required")
	}
	return projection.DeleteOldLog(ctx, targetTimestamp, 100)
}

func ListAllQuotaDates(startTimestamp int64, endTimestamp int64, username string) ([]*auditdomain.QuotaData, error) {
	return projection.GetAllQuotaDates(startTimestamp, endTimestamp, username)
}

func ListQuotaDatesByUser(startTimestamp int64, endTimestamp int64) ([]*auditdomain.QuotaData, error) {
	return projection.GetQuotaDataGroupByUser(startTimestamp, endTimestamp)
}

func ListUserQuotaDates(userID int, startTimestamp int64, endTimestamp int64) ([]*auditdomain.QuotaData, error) {
	if endTimestamp-startTimestamp > maxUserQuotaRangeSeconds {
		return nil, errors.New("时间跨度不能超过 1 个月")
	}
	return projection.GetQuotaDataByUserID(userID, startTimestamp, endTimestamp)
}
