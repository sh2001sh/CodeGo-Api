package http

import (
	"fmt"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
)

func GetPerformanceStats(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    adminopsapp.BuildPerformanceStats(),
	})
}

func ClearDiskCache(c *gin.Context) {
	if err := adminopsapp.CleanupInactiveDiskCache(); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "不活跃的磁盘缓存已清理",
	})
}

func ResetPerformanceStats(c *gin.Context) {
	adminopsapp.ResetPerformanceStats()
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "统计信息已重置",
	})
}

func ForceGC(c *gin.Context) {
	adminopsapp.ForceGC()
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "GC 已执行",
	})
}

func GetLogFiles(c *gin.Context) {
	response, err := adminopsapp.BuildLogFilesResponse()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, response)
}

func CleanupLogFiles(c *gin.Context) {
	mode := c.Query("mode")
	value, err := strconv.Atoi(c.Query("value"))
	if err != nil {
		httpapi.ApiErrorMsg(c, "invalid value, must be a positive integer")
		return
	}

	result, partialFailure, err := adminopsapp.CleanupLogFiles(mode, value)
	if err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}

	if partialFailure {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("部分文件删除失败（%d/%d）", len(result.FailedFiles), result.DeletedCount+len(result.FailedFiles)),
			"data":    result,
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
