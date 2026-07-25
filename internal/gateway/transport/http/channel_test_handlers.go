package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
)

func TestChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	testModel := c.Query("model")
	endpointType := c.Query("endpoint_type")
	isStream, _ := strconv.ParseBool(c.Query("stream"))
	consumedTime, apiError, localErr := gatewayexecutionapp.TestChannelByID(channelID, testModel, endpointType, isStream)
	if localErr != nil {
		resp := gin.H{
			"success": false,
			"message": localErr.Error(),
			"time":    0.0,
		}
		if apiError != nil {
			resp["error_code"] = apiError.GetErrorCode()
		}
		c.JSON(stdhttp.StatusOK, resp)
		return
	}
	if apiError != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success":    false,
			"message":    apiError.Error(),
			"time":       consumedTime,
			"error_code": apiError.GetErrorCode(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}

func TestAllChannels(c *gin.Context) {
	if err := gatewayexecutionapp.TestAllChannels(true); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
