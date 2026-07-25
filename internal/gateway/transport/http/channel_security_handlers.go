package http

import (
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
)

func GetChannelKey(c *gin.Context) {
	userID := c.GetInt("id")
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, fmt.Errorf("渠道ID格式错误: %v", err))
		return
	}

	result, err := gatewayexecutionapp.GetChannelKey(userID, channelID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "获取成功",
		"data":    result,
	})
}

func RefreshCodexChannelCredential(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	result, err := gatewayexecutionapp.RefreshCodexCredential(channelID)
	if err != nil {
		platformobservability.SysError("failed to refresh codex channel credential: " + err.Error())
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "刷新凭证失败，请稍后重试",
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "refreshed",
		"data":    result,
	})
}
