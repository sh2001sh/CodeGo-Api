package http

import (
	"github.com/gin-gonic/gin"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	query := readLogListQuery(c, pageInfo.GetStartIdx(), pageInfo.GetPageSize())

	logs, total, err := auditapp.ListAdminLogs(query)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetUserLogs(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	query := readLogListQuery(c, pageInfo.GetStartIdx(), pageInfo.GetPageSize())

	logs, total, err := auditapp.ListUserLogs(c.GetInt("id"), query)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	httpapi.ApiSuccess(c, pageInfo)
}

func SearchAllLogs(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func SearchUserLogs(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenID := c.GetInt("token_id")
	if tokenID == 0 {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}

	logs, err := auditapp.GetLogsByTokenID(tokenID)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	stat, err := auditapp.GetAdminLogStats(readLogListQuery(c, 0, 0))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
}

func GetLogsSelfStat(c *gin.Context) {
	stat, err := auditapp.GetUserLogStats(c.GetString("username"), readLogListQuery(c, 0, 0))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	count, err := auditapp.DeleteHistoryLogs(c.Request.Context(), targetTimestamp)
	if err != nil {
		if err.Error() == "target timestamp is required" {
			c.JSON(stdhttp.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetAllQuotaDates(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	dates, err := auditapp.ListAllQuotaDates(startTimestamp, endTimestamp, c.Query("username"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func GetQuotaDatesByUser(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	dates, err := auditapp.ListQuotaDatesByUser(startTimestamp, endTimestamp)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func GetUserQuotaDates(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	dates, err := auditapp.ListUserQuotaDates(c.GetInt("id"), startTimestamp, endTimestamp)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func readLogListQuery(c *gin.Context, startIdx int, pageSize int) auditapp.LogListQuery {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))

	return auditapp.LogListQuery{
		LogType:           logType,
		StartTimestamp:    startTimestamp,
		EndTimestamp:      endTimestamp,
		Username:          c.Query("username"),
		TokenName:         c.Query("token_name"),
		ModelName:         c.Query("model_name"),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestID:         c.Query("request_id"),
		UpstreamRequestID: c.Query("upstream_request_id"),
		StartIdx:          startIdx,
		PageSize:          pageSize,
	}
}
