package http

import (
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := parsePerfMetricsHours(c.Query("hours"))

	result, err := auditapp.BuildPerfMetricsSummary(hours)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetrics(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	result, err := auditapp.BuildPerfMetrics(modelName, c.Query("group"), parsePerfMetricsHours(c.Query("hours")))
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func parsePerfMetricsHours(raw string) int {
	hours := 24
	if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
		hours = parsed
	}
	return hours
}
