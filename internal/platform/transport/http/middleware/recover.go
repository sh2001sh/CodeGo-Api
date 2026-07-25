package middleware

import (
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				platformobservability.SysLog(fmt.Sprintf("panic detected: %v", err))
				platformobservability.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Panic detected, error: %v. Please submit an issue here: https://github.com/sh2001sh/CodeGo-Api", err),
						"type":    "codego_api_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
