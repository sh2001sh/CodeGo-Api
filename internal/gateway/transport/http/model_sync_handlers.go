package http

import (
	"errors"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"net/http"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func SyncUpstreamModels(c *gin.Context) {
	var req gatewayroutingapp.SyncRequest
	_ = c.ShouldBindJSON(&req)

	result, err := gatewayroutingapp.SyncUpstreamModels(c.Request.Context(), req)
	if err != nil {
		writeSyncError(c, req.Locale, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"created_models":  result.CreatedModels,
			"created_vendors": result.CreatedVendors,
			"updated_models":  result.UpdatedModels,
			"skipped_models":  result.SkippedModels,
			"created_list":    result.CreatedList,
			"updated_list":    result.UpdatedList,
			"source": gin.H{
				"locale":      result.Source.Locale,
				"models_url":  result.Source.ModelsURL,
				"vendors_url": result.Source.VendorsURL,
			},
		},
	})
}

func SyncUpstreamPreview(c *gin.Context) {
	locale := c.Query("locale")
	result, err := gatewayroutingapp.PreviewUpstreamModels(c.Request.Context(), locale)
	if err != nil {
		writeSyncError(c, locale, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"missing":   result.Missing,
			"conflicts": result.Conflicts,
			"source": gin.H{
				"locale":      result.Source.Locale,
				"models_url":  result.Source.ModelsURL,
				"vendors_url": result.Source.VendorsURL,
			},
		},
	})
}

func writeSyncError(c *gin.Context, locale string, err error) {
	var fetchErr *gatewayroutingapp.UpstreamFetchError
	if errors.As(err, &fetchErr) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取上游模型失败: " + fetchErr.Err.Error(),
			"locale":  locale,
			"source_urls": gin.H{
				"models_url":  fetchErr.Source.ModelsURL,
				"vendors_url": fetchErr.Source.VendorsURL,
			},
		})
		return
	}

	if errors.Is(err, gatewayroutingapp.ErrLoadMissingModels) {
		platformobservability.SysError("failed to get missing models: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取模型列表失败，请稍后重试",
		})
		return
	}

	platformobservability.SysError("failed to sync upstream models: " + err.Error())
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": err.Error(),
	})
}
