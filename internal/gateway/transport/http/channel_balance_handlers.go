package http

import (
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
)

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	balance, err := gatewayexecutionapp.UpdateChannelBalance(id)
	if errors.Is(err, gatewayexecutionapp.ErrMultiKeyChannelBalanceUnsupported) {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
	})
}

func UpdateAllChannelsBalance(c *gin.Context) {
	if err := gatewayexecutionapp.UpdateAllChannelsBalance(); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
