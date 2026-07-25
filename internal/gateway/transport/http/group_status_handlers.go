package http

import (
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func GetUserGroupStatus(c *gin.Context) {
	userID := c.GetInt("id")
	_, hasUser := c.Get("id")

	data, err := gatewayroutingapp.BuildUserGroupStatus(userID, hasUser)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
