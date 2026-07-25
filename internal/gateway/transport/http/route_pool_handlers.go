package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
)

type saveRoutePoolRequest struct {
	ID           int64                           `json:"id"`
	Name         string                          `json:"name"`
	Group        string                          `json:"group"`
	Enabled      bool                            `json:"enabled"`
	AutoDiscover bool                            `json:"auto_discover"`
	Members      []gatewayschema.RoutePoolMember `json:"members"`
}

type saveRoutePoolGroupRequest struct {
	Group   string                          `json:"group"`
	Enabled bool                            `json:"enabled"`
	Members []gatewayschema.RoutePoolMember `json:"members"`
}

func ListRoutePoolGroups(c *gin.Context) {
	groups, err := gatewayroutingapp.ListRoutePoolGroups()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, gin.H{"items": groups})
}

func SaveRoutePoolGroup(c *gin.Context) {
	var request saveRoutePoolGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pool, err := gatewayroutingapp.SaveRoutePoolGroup(request.Group, request.Enabled, request.Members)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, pool)
}

func ListRoutePools(c *gin.Context) {
	pools, err := gatewayroutingapp.ListRoutePools()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, gin.H{"items": pools})
}

func SaveRoutePool(c *gin.Context) {
	var request saveRoutePoolRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pool, err := gatewayroutingapp.SaveRoutePool(gatewayschema.RoutePool{
		ID: request.ID, Name: request.Name, Group: request.Group, Enabled: request.Enabled, AutoDiscover: request.AutoDiscover,
	}, request.Members)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, pool)
}

func DeleteRoutePool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := gatewayroutingapp.DeleteRoutePool(id); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func GetRoutePoolMetrics(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	metrics, err := gatewayroutingapp.GetRoutePoolMetrics(id, c.Query("model"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, metrics)
}
