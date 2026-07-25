package http

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
	"strconv"
)

func GetAllUsers(c *gin.Context) {
	pageInfo, err := identityapp.ListAdminUsers(platformpagination.GetPageQuery(c))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, pageInfo)
}

func SearchUsers(c *gin.Context) {
	pageInfo, err := identityapp.SearchAdminUsers(c.Query("keyword"), c.Query("group"), platformpagination.GetPageQuery(c))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, pageInfo)
}

func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	user, err := identityapp.GetAdminUserDetail(id, c.GetInt("role"))
	if err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": user})
}

func CreateUser(c *gin.Context) {
	var req identityapp.AdminUserMutateRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := identityapp.CreateAdminUser(req, c.GetInt("role")); err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func UpdateUser(c *gin.Context) {
	var req identityapp.AdminUserMutateRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := identityapp.UpdateAdminUser(req, c.GetInt("role")); err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if err := identityapp.DeleteAdminUser(id, c.GetInt("role")); err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func ManageUser(c *gin.Context) {
	var req identityapp.AdminUserManageRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	result, err := identityapp.ManageAdminUser(req, identityapp.AdminActionActor{
		UserID:   c.GetInt("id"),
		Username: c.GetString("username"),
		Role:     c.GetInt("role"),
	})
	if err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func AdminClearUserBinding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := identityapp.ClearAdminUserBinding(id, c.Param("binding_type"), c.GetInt("role")); err != nil {
		handleAdminUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success"})
}

func handleAdminUserError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, identityapp.ErrInvalidParams):
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
	case errors.Is(err, identityapp.ErrUserNotExists):
		httpapi.ApiErrorI18n(c, i18n.MsgUserNotExists)
	case errors.Is(err, identityapp.ErrUserNoPermissionSame):
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
	case errors.Is(err, identityapp.ErrUserNoPermissionHigher):
		httpapi.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
	case errors.Is(err, identityapp.ErrUserCannotCreateHigher):
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotCreateHigherLevel)
	case errors.Is(err, identityapp.ErrUserCannotDeleteRoot):
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDeleteRootUser)
	case errors.Is(err, identityapp.ErrUserCannotDisableRoot):
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDisableRootUser)
	case errors.Is(err, identityapp.ErrUserCannotDemoteRoot):
		httpapi.ApiErrorI18n(c, i18n.MsgUserCannotDemoteRootUser)
	case errors.Is(err, identityapp.ErrUserAlreadyAdmin):
		httpapi.ApiErrorI18n(c, i18n.MsgUserAlreadyAdmin)
	case errors.Is(err, identityapp.ErrUserAlreadyCommon):
		httpapi.ApiErrorI18n(c, i18n.MsgUserAlreadyCommon)
	case errors.Is(err, identityapp.ErrUserAdminCannotPromote):
		httpapi.ApiErrorI18n(c, i18n.MsgUserAdminCannotPromote)
	case errors.Is(err, identityapp.ErrUserQuotaChangeZero):
		httpapi.ApiErrorI18n(c, i18n.MsgUserQuotaChangeZero)
	default:
		httpapi.ApiError(c, err)
	}
}
