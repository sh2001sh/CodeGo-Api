package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"strconv"

	"github.com/gin-gonic/gin"

	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
)

// GetAllVendors returns paginated vendors.

func GetAllVendors(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	vendors, total, err := adminopsapp.ListVendors(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(vendors)
	httpapi.ApiSuccess(c, pageInfo)
}

// SearchVendors returns paginated vendor search results.
func SearchVendors(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	vendors, total, err := adminopsapp.SearchVendors(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(vendors)
	httpapi.ApiSuccess(c, pageInfo)
}

// GetVendorMeta returns one vendor by ID.
func GetVendorMeta(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrVendorIDInvalid.Error())
		return
	}
	vendor, err := adminopsapp.GetVendorMeta(id)
	if err != nil {
		handleVendorMetaError(c, err)
		return
	}
	httpapi.ApiSuccess(c, vendor)
}

// CreateVendorMeta creates a vendor.
func CreateVendorMeta(c *gin.Context) {
	var vendor gatewayschema.Vendor
	if err := c.ShouldBindJSON(&vendor); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	created, err := adminopsapp.CreateVendorMeta(vendor)
	if err != nil {
		handleVendorMetaError(c, err)
		return
	}
	httpapi.ApiSuccess(c, created)
}

// UpdateVendorMeta updates a vendor.
func UpdateVendorMeta(c *gin.Context) {
	var vendor gatewayschema.Vendor
	if err := c.ShouldBindJSON(&vendor); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	updated, err := adminopsapp.UpdateVendorMeta(vendor)
	if err != nil {
		handleVendorMetaError(c, err)
		return
	}
	httpapi.ApiSuccess(c, updated)
}

// DeleteVendorMeta deletes a vendor.
func DeleteVendorMeta(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrVendorIDInvalid.Error())
		return
	}
	if err := adminopsapp.DeleteVendorMeta(id); err != nil {
		handleVendorMetaError(c, err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func handleVendorMetaError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case adminopsapp.ErrVendorIDInvalid,
		adminopsapp.ErrVendorIDRequired,
		adminopsapp.ErrVendorNameRequired,
		adminopsapp.ErrVendorNameDuplicated:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
