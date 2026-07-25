package http

import (
	"github.com/gin-gonic/gin"
	workflowapp "github.com/sh2001sh/CodeGo-Api/internal/workflow/app"
)

// VideoProxy forwards authenticated video content proxy requests into the workflow module.
func VideoProxy(c *gin.Context) {
	workflowapp.ProxyVideoContent(c)
}
