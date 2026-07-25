package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func ApplyChannelUpstreamModelUpdates(c *gin.Context) {
	var req gatewayroutingapp.ApplyChannelUpstreamModelUpdatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if req.ID <= 0 {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "invalid channel id",
		})
		return
	}

	result, err := gatewayroutingapp.ApplyChannelUpstreamModelUpdates(req.ID, req.AddModels, req.IgnoreModels, req.RemoveModels)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":                      result.ChannelID,
			"added_models":            result.AddedModels,
			"removed_models":          result.RemovedModels,
			"ignored_models":          result.IgnoredModels,
			"remaining_models":        result.RemainingModels,
			"remaining_remove_models": result.RemainingRemoveModels,
			"models":                  result.Models,
			"settings":                result.Settings,
		},
	})
}

func DetectChannelUpstreamModelUpdates(c *gin.Context) {
	var req gatewayroutingapp.ApplyChannelUpstreamModelUpdatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if req.ID <= 0 {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "invalid channel id",
		})
		return
	}

	result, err := gatewayroutingapp.DetectChannelUpstreamModelUpdates(req.ID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

func ApplyAllChannelUpstreamModelUpdates(c *gin.Context) {
	result, err := gatewayroutingapp.ApplyAllChannelUpstreamModelUpdates()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"processed_channels": result.ProcessedChannels,
			"added_models":       result.AddedModels,
			"removed_models":     result.RemovedModels,
			"failed_channel_ids": result.FailedChannelIDs,
			"results":            result.Results,
		},
	})
}

func DetectAllChannelUpstreamModelUpdates(c *gin.Context) {
	result, err := gatewayroutingapp.DetectAllChannelUpstreamModelUpdates()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"processed_channels":       result.ProcessedChannels,
			"failed_channel_ids":       result.FailedChannelIDs,
			"detected_add_models":      result.DetectedAddModels,
			"detected_remove_models":   result.DetectedRemoveModels,
			"channel_detected_results": result.ChannelDetectedResults,
		},
	})
}
