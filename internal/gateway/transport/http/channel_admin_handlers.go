package http

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func invalidParams(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": false,
		"message": "参数错误",
	})
}

func DeleteDisabledChannels(c *gin.Context) {
	rows, err := gatewayroutingapp.DeleteDisabledChannels()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

func DisableTagChannels(c *gin.Context) {
	var req gatewayroutingapp.ChannelTagUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Tag == "" {
		invalidParams(c)
		return
	}
	if err := gatewayroutingapp.DisableChannelsByTag(req.Tag); err != nil {
		if err.Error() == "tag不能为空" {
			invalidParams(c)
			return
		}
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EnableTagChannels(c *gin.Context) {
	var req gatewayroutingapp.ChannelTagUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Tag == "" {
		invalidParams(c)
		return
	}
	if err := gatewayroutingapp.EnableChannelsByTag(req.Tag); err != nil {
		if err.Error() == "tag不能为空" {
			invalidParams(c)
			return
		}
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EditTagChannels(c *gin.Context) {
	var req gatewayroutingapp.ChannelTagUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidParams(c)
		return
	}
	if err := gatewayroutingapp.EditChannelsByTag(req); err != nil {
		switch err.Error() {
		case "tag不能为空", "参数覆盖必须是合法的 JSON 格式", "请求头覆盖必须是合法的 JSON 格式":
			c.JSON(stdhttp.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		default:
			httpapi.ApiError(c, err)
			return
		}
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteChannelBatch(c *gin.Context) {
	var req gatewayroutingapp.ChannelBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidParams(c)
		return
	}
	count, err := gatewayroutingapp.DeleteChannelsBatch(req.IDs)
	if err != nil {
		if err.Error() == "参数错误" {
			invalidParams(c)
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

func BatchSetChannelTag(c *gin.Context) {
	var req gatewayroutingapp.ChannelBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidParams(c)
		return
	}
	count, err := gatewayroutingapp.BatchSetChannelsTag(req.IDs, req.Tag)
	if err != nil {
		if err.Error() == "参数错误" {
			invalidParams(c)
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

func GetTagModels(c *gin.Context) {
	tag := c.Query("tag")
	if tag == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "tag不能为空",
		})
		return
	}
	models, err := gatewayroutingapp.GetLongestTagModels(tag)
	if err != nil {
		if err.Error() == "tag不能为空" {
			c.JSON(stdhttp.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    models,
	})
}

func CopyChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}

	suffix := c.DefaultQuery("suffix", "_复制")
	resetBalance := true
	if rbStr := c.DefaultQuery("reset_balance", "true"); rbStr != "" {
		if value, parseErr := strconv.ParseBool(rbStr); parseErr == nil {
			resetBalance = value
		}
	}

	clone, err := gatewayroutingapp.CopyChannel(id, suffix, resetBalance)
	if err != nil {
		platformobservability.SysError("failed to clone channel: " + err.Error())
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "复制渠道失败，请稍后重试",
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id": clone.Id,
		},
	})
}
