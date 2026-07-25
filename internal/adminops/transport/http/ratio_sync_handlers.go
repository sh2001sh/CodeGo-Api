package http

import (
	"errors"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
)

// GetSyncableChannels returns channels and built-in presets that support ratio sync.
func GetSyncableChannels(c *gin.Context) {
	channels, err := adminopsapp.ListSyncableChannels()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, channels)
}

// FetchUpstreamRatios compares local pricing data with upstream ratio config.
func FetchUpstreamRatios(c *gin.Context) {
	var req dto.UpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformobservability.SysError("failed to bind upstream request: " + err.Error())
		c.JSON(stdhttp.StatusBadRequest, gin.H{"success": false, "message": "请求参数格式错误"})
		return
	}

	result, err := adminopsapp.FetchUpstreamRatios(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, adminopsapp.ErrNoValidUpstreams):
			c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, adminopsapp.ErrQueryChannelsFailed):
			c.JSON(stdhttp.StatusInternalServerError, gin.H{"success": false, "message": adminopsapp.ErrQueryChannelsFailed.Error()})
		default:
			httpapi.ApiError(c, err)
		}
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
