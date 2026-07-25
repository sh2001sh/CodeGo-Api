package http

import (
	"errors"
	"fmt"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"strconv"

	"github.com/gin-gonic/gin"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
)

func GetImageWorkspaceModels(c *gin.Context) {
	models, err := identityapp.ListImageWorkspaceModels(c.GetInt("id"), c.Query("group"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, models)
}

func GetImageWorkspaceItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("p"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	payload, err := identityapp.ListImageWorkspaceItems(c.GetInt("id"), c.Query("session_id"), page, pageSize)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    payload.Items,
		"total":   payload.Total,
	})
}

func GetImageWorkspaceItemContent(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("id"))
	if err != nil || itemID <= 0 {
		httpapi.ApiErrorMsg(c, "invalid image item id")
		return
	}

	item, err := identityapp.LoadImageWorkspaceItemContentSource(c.GetInt("id"), itemID)
	if err != nil {
		switch {
		case errors.Is(err, identityapp.ErrImageWorkspaceItemNotFound):
			httpapi.ApiErrorMsg(c, "image not found")
			return
		case errors.Is(err, identityapp.ErrImageWorkspaceItemExpired):
			httpapi.ApiErrorMsg(c, "image has expired")
			return
		case errors.Is(err, identityapp.ErrImageWorkspaceFileMissing):
			httpapi.ApiErrorMsg(c, "image file not found")
			return
		default:
			httpapi.ApiError(c, err)
			return
		}
	}

	download := c.Query("download") == "1"
	if err := identityapp.WriteImageWorkspaceAsset(c, item, download); err != nil {
		httpapi.ApiError(c, fmt.Errorf("failed to read image: %w", err))
	}
}
