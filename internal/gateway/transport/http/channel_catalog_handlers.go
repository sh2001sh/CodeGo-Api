package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"
)

func parseTypeFilter(typeValue string) int {
	if typeValue == "" {
		return -1
	}
	channelType, err := strconv.Atoi(typeValue)
	if err != nil {
		return -1
	}
	return channelType
}

func GetAllChannels(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))
	sortOptions := modelSortOptions(c, idSort)

	result, err := gatewayroutingapp.ListChannels(gatewayroutingapp.ChannelListParams{
		Page:       pageInfo.GetPage(),
		PageSize:   pageInfo.GetPageSize(),
		EnableTag:  enableTagMode,
		Group:      c.Query("group"),
		Status:     gatewayroutingapp.ParseChannelStatusFilter(c.Query("status")),
		TypeFilter: parseTypeFilter(c.Query("type")),
		Sort:       sortOptions,
	})
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	httpapi.ApiSuccess(c, gin.H{
		"items":       result.Items,
		"total":       result.Total,
		"page":        result.Page,
		"page_size":   result.PageSize,
		"type_counts": result.TypeCounts,
	})
}

func modelSortOptions(c *gin.Context, idSort bool) gatewayroutingapp.ChannelSortOptions {
	return gatewayroutingapp.NewChannelSortOptions(c.Query("sort_by"), c.Query("sort_order"), idSort)
}

func SearchChannels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))

	result, err := gatewayroutingapp.SearchChannels(gatewayroutingapp.ChannelSearchParams{
		Keyword:    c.Query("keyword"),
		Group:      c.Query("group"),
		Model:      c.Query("model"),
		Status:     gatewayroutingapp.ParseChannelStatusFilter(c.Query("status")),
		TypeFilter: parseTypeFilter(c.Query("type")),
		EnableTag:  enableTagMode,
		Page:       page,
		PageSize:   pageSize,
		IDSort:     idSort,
		Sort:       modelSortOptions(c, idSort),
	})
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":       result.Items,
			"total":       result.Total,
			"type_counts": result.TypeCounts,
		},
	})
}

func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	channel, err := gatewayroutingapp.GetChannel(id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}

func FetchUpstreamModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	ids, err := gatewayroutingapp.FetchUpstreamModels(id)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %s", err.Error()),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ids,
	})
}

func FixChannelsAbilities(c *gin.Context) {
	successCount, failCount, err := gatewayroutingapp.FixChannelAbilities()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"success": successCount,
			"fails":   failCount,
		},
	})
}

func AddChannel(c *gin.Context) {
	var req gatewayroutingapp.AddChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := gatewayroutingapp.AddChannel(req); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := gatewayroutingapp.DeleteChannel(id); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateChannel(c *gin.Context) {
	var patch gatewayroutingapp.ChannelPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	updated, err := gatewayroutingapp.UpdateChannel(patch)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updated,
	})
}

func FetchModels(c *gin.Context) {
	var req gatewayroutingapp.FetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}
	models, err := gatewayroutingapp.FetchRemoteModels(req)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}
