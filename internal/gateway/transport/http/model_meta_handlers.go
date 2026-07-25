package http

import (
	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
	"strconv"
)

func GetMissingModels(c *gin.Context) {
	missing, err := gatewayroutingapp.GetMissingModels()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    missing,
	})
}

func GetAllModelsMeta(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	status := c.Query("status")
	syncOfficial := c.Query("sync_official")
	result, err := gatewayroutingapp.GetAllModelsMeta(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), status, syncOfficial)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, gin.H{
		"items":         result.Items,
		"total":         result.Total,
		"page":          pageInfo.GetPage(),
		"page_size":     pageInfo.GetPageSize(),
		"vendor_counts": result.VendorCounts,
	})
}

func SearchModelsMeta(c *gin.Context) {
	keyword := c.Query("keyword")
	vendor := c.Query("vendor")
	status := c.Query("status")
	syncOfficial := c.Query("sync_official")
	pageInfo := platformpagination.GetPageQuery(c)

	items, total, err := gatewayroutingapp.SearchModelsMeta(
		keyword,
		vendor,
		status,
		syncOfficial,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetModelMeta(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	item, err := gatewayroutingapp.GetModelMeta(id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, item)
}

func CreateModelMeta(c *gin.Context) {
	var item gatewayschema.Model
	if err := c.ShouldBindJSON(&item); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if item.ModelName == "" {
		httpapi.ApiErrorMsg(c, "模型名称不能为空")
		return
	}
	if dup, err := gatewayroutingapp.IsModelNameDuplicated(0, item.ModelName); err != nil {
		httpapi.ApiError(c, err)
		return
	} else if dup {
		httpapi.ApiErrorMsg(c, "模型名称已存在")
		return
	}
	if err := gatewayroutingapp.CreateModelMeta(&item); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, &item)
}

func UpdateModelMeta(c *gin.Context) {
	statusOnly := c.Query("status_only") == "true"

	var item gatewayschema.Model
	if err := c.ShouldBindJSON(&item); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if item.Id == 0 {
		httpapi.ApiErrorMsg(c, "缺少模型 ID")
		return
	}
	if !statusOnly {
		if dup, err := gatewayroutingapp.IsModelNameDuplicated(item.Id, item.ModelName); err != nil {
			httpapi.ApiError(c, err)
			return
		} else if dup {
			httpapi.ApiErrorMsg(c, "模型名称已存在")
			return
		}
	}
	if err := gatewayroutingapp.UpdateModelMeta(&item, statusOnly); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, &item)
}

func DeleteModelMeta(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := gatewayroutingapp.DeleteModelMeta(id); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}
