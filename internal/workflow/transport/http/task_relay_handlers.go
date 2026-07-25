package http

import (
	"github.com/gin-gonic/gin"
	workflowapp "github.com/sh2001sh/CodeGo-Api/internal/workflow/app"
)

// SubmitRelayTask forwards async relay submission requests into the workflow module.
func SubmitRelayTask(c *gin.Context) {
	workflowapp.SubmitRelayTask(c)
}

// FetchRelayTask forwards async relay fetch requests into the workflow module.
func FetchRelayTask(c *gin.Context) {
	workflowapp.FetchRelayTask(c)
}
