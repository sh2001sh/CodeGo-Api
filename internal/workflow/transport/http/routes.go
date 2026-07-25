package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterWorkflowRoutes(apiRouter *gin.RouterGroup) {
	taskRoute := apiRouter.Group("/task")
	{
		taskRoute.GET("/self", middleware.UserAuth(), GetUserTask)
		taskRoute.GET("/", middleware.AdminAuth(), GetAllTask)
	}

}
