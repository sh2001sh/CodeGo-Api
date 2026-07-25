package middleware

import (
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	userId := c.GetInt("id")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": platformtext.MessageWithRequestID(message, c.GetString(constant.RequestIdKey)),
			"type":    "codego_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}
