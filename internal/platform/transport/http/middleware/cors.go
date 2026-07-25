package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-CodeGo-Api-Version", platformconfig.Version)
		c.Next()
	}
}
