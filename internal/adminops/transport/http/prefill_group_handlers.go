package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"strconv"

	"github.com/gin-gonic/gin"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
)

// GetPrefillGroups returns prefill groups optionally filtered by type.
func GetPrefillGroups(c *gin.Context) {
	groups, err := adminopsapp.ListPrefillGroups(c.Query("type"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, groups)
}

// CreatePrefillGroup creates a prefill group.
func CreatePrefillGroup(c *gin.Context) {
	var group gatewayschema.PrefillGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	created, err := adminopsapp.CreatePrefillGroup(group)
	if err != nil {
		handlePrefillGroupError(c, err)
		return
	}
	httpapi.ApiSuccess(c, created)
}

// UpdatePrefillGroup updates a prefill group.
func UpdatePrefillGroup(c *gin.Context) {
	var group gatewayschema.PrefillGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	updated, err := adminopsapp.UpdatePrefillGroup(group)
	if err != nil {
		handlePrefillGroupError(c, err)
		return
	}
	httpapi.ApiSuccess(c, updated)
}

// DeletePrefillGroup deletes a prefill group.
func DeletePrefillGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrPrefillGroupIDInvalid.Error())
		return
	}
	if err := adminopsapp.DeletePrefillGroup(id); err != nil {
		handlePrefillGroupError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func handlePrefillGroupError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case adminopsapp.ErrPrefillGroupIDInvalid,
		adminopsapp.ErrPrefillGroupIDRequired,
		adminopsapp.ErrPrefillGroupNameOrTypeRequired,
		adminopsapp.ErrPrefillGroupNameDuplicated:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
