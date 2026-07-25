package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sh2001sh/CodeGo-Api/i18n"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
)

// GetAllRedemptions returns paginated redemptions.

func GetAllRedemptions(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	redemptions, total, err := adminopsapp.ListRedemptions(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	httpapi.ApiSuccess(c, pageInfo)
}

// SearchRedemptions returns paginated redemption search results.
func SearchRedemptions(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	redemptions, total, err := adminopsapp.SearchRedemptions(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	httpapi.ApiSuccess(c, pageInfo)
}

// GetRedemption returns a redemption by ID.
func GetRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrRedemptionIDInvalid.Error())
		return
	}
	redemption, err := adminopsapp.GetRedemption(id)
	if err != nil {
		handleRedemptionError(c, err)
		return
	}
	httpapi.ApiSuccess(c, redemption)
}

// AddRedemption creates new redemption codes.
func AddRedemption(c *gin.Context) {
	if !adminopsapp.IsPaymentComplianceConfirmed() {
		httpapi.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}

	var redemption commerceschema.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	result, err := adminopsapp.CreateRedemption(c.GetInt("id"), redemption)
	if err != nil {
		handleRedemptionError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result.Keys,
	})
}

// UpdateRedemption updates a redemption entry.
func UpdateRedemption(c *gin.Context) {
	var redemption commerceschema.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		httpapi.ApiError(c, err)
		return
	}
	updated, err := adminopsapp.UpdateRedemption(redemption, c.Query("status_only") != "")
	if err != nil {
		handleRedemptionError(c, err)
		return
	}
	httpapi.ApiSuccess(c, updated)
}

// DeleteRedemption deletes one redemption.
func DeleteRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrRedemptionIDInvalid.Error())
		return
	}
	if err := adminopsapp.DeleteRedemption(id); err != nil {
		handleRedemptionError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// DeleteInvalidRedemption deletes used, disabled, and expired redemption codes.
func DeleteInvalidRedemption(c *gin.Context) {
	rows, err := adminopsapp.DeleteInvalidRedemptions()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, rows)
}

func handleRedemptionError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case adminopsapp.ErrRedemptionIDInvalid:
		httpapi.ApiErrorMsg(c, err.Error())
	case adminopsapp.ErrRedemptionNameLengthInvalid:
		httpapi.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
	case adminopsapp.ErrRedemptionCountPositiveRequired:
		httpapi.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
	case adminopsapp.ErrRedemptionCountMaxExceeded:
		httpapi.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
	case adminopsapp.ErrRedemptionExpireTimeInvalid:
		httpapi.ApiErrorI18n(c, i18n.MsgRedemptionExpireTimeInvalid)
	case adminopsapp.ErrRedemptionQuotaRequired,
		adminopsapp.ErrRedemptionSubscriptionPlanInvalid,
		adminopsapp.ErrRedemptionSubscriptionPlanMissing,
		adminopsapp.ErrRedemptionPayloadEmpty:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
