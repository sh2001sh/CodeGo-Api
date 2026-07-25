package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"gorm.io/gorm"
	stdhttp "net/http"
	"strconv"
)

func bindOllamaModelRequest(c *gin.Context) (*gatewayexecutionapp.OllamaModelRequest, bool) {
	var req gatewayexecutionapp.OllamaModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
		})
		return nil, false
	}
	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "Channel ID and model name are required",
		})
		return nil, false
	}
	return &req, true
}

func mapOllamaChannelError(err error) (int, string) {
	if err == nil {
		return stdhttp.StatusOK, ""
	}
	if err.Error() == "This operation is only supported for Ollama channels" {
		return stdhttp.StatusBadRequest, err.Error()
	}
	if err == gorm.ErrRecordNotFound {
		return stdhttp.StatusNotFound, "Channel not found"
	}
	return stdhttp.StatusInternalServerError, err.Error()
}

func PullOllamaModel(c *gin.Context) {
	req, ok := bindOllamaModelRequest(c)
	if !ok {
		return
	}
	if err := gatewayexecutionapp.PullOllamaModel(*req); err != nil {
		status, message := mapOllamaChannelError(err)
		c.JSON(status, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to pull model: %s", message),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Model %s pulled successfully", req.ModelName),
	})
}

func PullOllamaModelStream(c *gin.Context) {
	req, ok := bindOllamaModelRequest(c)
	if !ok {
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	err := gatewayexecutionapp.StreamPullOllamaModel(*req, func(event gatewayexecutionapp.OllamaStreamEvent) {
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(event.Data))
		c.Writer.Flush()
	})
	if err != nil {
		errorData, _ := platformencoding.Marshal(gin.H{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(errorData))
	} else {
		successData, _ := platformencoding.Marshal(gin.H{"message": fmt.Sprintf("Model %s pulled successfully", req.ModelName)})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(successData))
	}
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

func DeleteOllamaModel(c *gin.Context) {
	req, ok := bindOllamaModelRequest(c)
	if !ok {
		return
	}
	if err := gatewayexecutionapp.DeleteOllamaModel(*req); err != nil {
		status, message := mapOllamaChannelError(err)
		c.JSON(status, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to delete model: %s", message),
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Model %s deleted successfully", req.ModelName),
	})
}

func OllamaVersion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid channel id",
		})
		return
	}
	version, err := gatewayexecutionapp.GetOllamaVersion(id)
	if err != nil {
		status, message := mapOllamaChannelError(err)
		if status == stdhttp.StatusInternalServerError {
			c.JSON(stdhttp.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取Ollama版本失败: %s", message),
			})
			return
		}
		c.JSON(status, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"version": version,
		},
	})
}
