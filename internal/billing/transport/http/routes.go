package http

import (
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

func RegisterBillingRoutes(apiRouter *gin.RouterGroup) {
	route := apiRouter.Group("/billing")
	route.Use(middleware.AdminAuth())
	route.GET("/reconciliations", listReconciliations)
	route.POST("/reconciliations/:account_id/repair", repairReconciliation)
	route.GET("/operations", operationalSLO)
}

func operationalSLO(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	result, err := billingapp.BuildOperationalSLO(c.Request.Context(), hours)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"success": true, "data": result})
}

func listReconciliations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := billingapp.ListLedgerReconciliations(c.Request.Context(), limit)
	if err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"success": true, "data": items})
}

func repairReconciliation(c *gin.Context) {
	if err := billingapp.RepairLedgerSnapshot(c.Request.Context(), c.Param("account_id")); err != nil {
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{"success": true})
}
