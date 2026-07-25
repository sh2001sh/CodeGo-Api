package app

import (
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	platformvalidation "github.com/sh2001sh/CodeGo-Api/internal/platform/validation"
	"strings"
)

var (
	ErrUserNotExists          = errors.New(i18n.MsgUserNotExists)
	ErrUserNoPermissionSame   = errors.New(i18n.MsgUserNoPermissionSameLevel)
	ErrUserNoPermissionHigher = errors.New(i18n.MsgUserNoPermissionHigherLevel)
	ErrUserCannotCreateHigher = errors.New(i18n.MsgUserCannotCreateHigherLevel)
	ErrUserCannotDeleteRoot   = errors.New(i18n.MsgUserCannotDeleteRootUser)
	ErrUserCannotDisableRoot  = errors.New(i18n.MsgUserCannotDisableRootUser)
	ErrUserCannotDemoteRoot   = errors.New(i18n.MsgUserCannotDemoteRootUser)
	ErrUserAlreadyAdmin       = errors.New(i18n.MsgUserAlreadyAdmin)
	ErrUserAlreadyCommon      = errors.New(i18n.MsgUserAlreadyCommon)
	ErrUserAdminCannotPromote = errors.New(i18n.MsgUserAdminCannotPromote)
	ErrUserQuotaChangeZero    = errors.New(i18n.MsgUserQuotaChangeZero)
)

type AdminUserManageRequest struct {
	Id     int    `json:"id"`
	Action string `json:"action"`
	Value  int    `json:"value"`
	Mode   string `json:"mode"`
}

type AdminUserMutateRequest struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Group       string `json:"group"`
	Remark      string `json:"remark"`
	Role        int    `json:"role"`
}

type AdminActionActor struct {
	UserID   int
	Username string
	Role     int
}

type UserRoleStatusResponse struct {
	Role   int `json:"role"`
	Status int `json:"status"`
}

func ListAdminUsers(pageInfo *platformpagination.PageInfo) (*platformpagination.PageInfo, error) {
	users, total, err := identitystore.ListUsers(pageInfo)
	if err != nil {
		return nil, err
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	return pageInfo, nil
}

func SearchAdminUsers(keyword string, group string, pageInfo *platformpagination.PageInfo) (*platformpagination.PageInfo, error) {
	users, total, err := identitystore.SearchUsers(keyword, group, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		return nil, err
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	return pageInfo, nil
}

func GetAdminUserDetail(targetUserID int, actorRole int) (*identityschema.User, error) {
	user, err := LoadUserByID(targetUserID, false)
	if err != nil {
		return nil, err
	}
	if actorRole <= user.Role && actorRole != constant.RoleRootUser {
		return nil, ErrUserNoPermissionSame
	}
	return user, nil
}

func CreateAdminUser(req AdminUserMutateRequest, actorRole int) error {
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return ErrInvalidParams
	}

	candidate := identityschema.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Role:        req.Role,
	}
	if err := platformvalidation.Validate.Struct(&candidate); err != nil {
		return err
	}
	if candidate.DisplayName == "" {
		candidate.DisplayName = candidate.Username
	}
	if candidate.Role >= actorRole {
		return ErrUserCannotCreateHigher
	}

	cleanUser := identityschema.User{
		Username:    candidate.Username,
		Password:    candidate.Password,
		DisplayName: candidate.DisplayName,
		Role:        candidate.Role,
	}
	return createUserAndRecordRegistration(&cleanUser)
}

func UpdateAdminUser(req AdminUserMutateRequest, actorRole int) error {
	if req.Id == 0 {
		return ErrInvalidParams
	}

	updatedUser := identityschema.User{
		Id:          req.Id,
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Group:       req.Group,
		Remark:      req.Remark,
		Role:        req.Role,
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U"
	}
	if err := platformvalidation.Validate.Struct(&updatedUser); err != nil {
		return err
	}

	originUser, err := LoadUserByID(updatedUser.Id, false)
	if err != nil {
		return err
	}
	if actorRole <= originUser.Role && actorRole != constant.RoleRootUser {
		return ErrUserNoPermissionHigher
	}
	if actorRole <= updatedUser.Role && actorRole != constant.RoleRootUser {
		return ErrUserCannotCreateHigher
	}

	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = ""
	}
	return identitystore.EditUser(&updatedUser, updatedUser.Password != "")
}

func DeleteAdminUser(targetUserID int, actorRole int) error {
	originUser, err := LoadUserByID(targetUserID, false)
	if err != nil {
		return err
	}
	if actorRole <= originUser.Role {
		return ErrUserNoPermissionHigher
	}
	return identitystore.HardDeleteUserByID(targetUserID)
}

func ClearAdminUserBinding(targetUserID int, bindingType string, actorRole int) error {
	if strings.TrimSpace(bindingType) == "" {
		return ErrInvalidParams
	}

	user, err := LoadUserByID(targetUserID, false)
	if err != nil {
		return err
	}
	if actorRole <= user.Role && actorRole != constant.RoleRootUser {
		return ErrUserNoPermissionSame
	}
	normalizedBindingType := strings.ToLower(strings.TrimSpace(bindingType))
	clearedUser, err := identitystore.ClearUserBinding(user.Id, normalizedBindingType)
	if err != nil {
		return err
	}
	auditapp.RecordLog(clearedUser.Id, auditschema.LogTypeManage, fmt.Sprintf("admin cleared %s binding for user %s", normalizedBindingType, clearedUser.Username))
	return nil
}

func ManageAdminUser(req AdminUserManageRequest, actor AdminActionActor) (*UserRoleStatusResponse, error) {
	if req.Id == 0 || strings.TrimSpace(req.Action) == "" {
		return nil, ErrInvalidParams
	}

	user := identityschema.User{Id: req.Id}
	platformdb.DB.Unscoped().Where(&user).First(&user)
	if user.Id == 0 {
		return nil, ErrUserNotExists
	}
	if actor.Role <= user.Role && actor.Role != constant.RoleRootUser {
		return nil, ErrUserNoPermissionHigher
	}

	switch req.Action {
	case "disable":
		if user.Role == constant.RoleRootUser {
			return nil, ErrUserCannotDisableRoot
		}
		user.Status = constant.UserStatusDisabled
	case "enable":
		user.Status = constant.UserStatusEnabled
	case "delete":
		if user.Role == constant.RoleRootUser {
			return nil, ErrUserCannotDeleteRoot
		}
		if err := identitystore.DeleteUserByID(user.Id); err != nil {
			return nil, err
		}
		if err := identitystore.InvalidateUserTokensCache(user.Id); err != nil {
			platformobservability.SysLog(fmt.Sprintf("failed to invalidate tokens cache for user %d: %s", user.Id, err.Error()))
		}
		return nil, nil
	case "promote":
		if actor.Role != constant.RoleRootUser {
			return nil, ErrUserAdminCannotPromote
		}
		if user.Role >= constant.RoleAdminUser {
			return nil, ErrUserAlreadyAdmin
		}
		user.Role = constant.RoleAdminUser
	case "demote":
		if user.Role == constant.RoleRootUser {
			return nil, ErrUserCannotDemoteRoot
		}
		if user.Role == constant.RoleCommonUser {
			return nil, ErrUserAlreadyCommon
		}
		user.Role = constant.RoleCommonUser
	case "add_quota":
		if err := applyAdminQuotaChange(user, req, actor); err != nil {
			return nil, err
		}
		return &UserRoleStatusResponse{Role: user.Role, Status: user.Status}, nil
	default:
		return nil, ErrInvalidParams
	}

	if err := identitystore.UpdateUser(&user, false); err != nil {
		return nil, err
	}
	if req.Action == "disable" || req.Action == "promote" || req.Action == "demote" {
		if err := identitystore.InvalidateUserCache(user.Id); err != nil {
			platformobservability.SysLog(fmt.Sprintf("failed to invalidate user cache for user %d: %s", user.Id, err.Error()))
		}
		if err := identitystore.InvalidateUserTokensCache(user.Id); err != nil {
			platformobservability.SysLog(fmt.Sprintf("failed to invalidate tokens cache for user %d: %s", user.Id, err.Error()))
		}
	}
	return &UserRoleStatusResponse{Role: user.Role, Status: user.Status}, nil
}

func applyAdminQuotaChange(user identityschema.User, req AdminUserManageRequest, actor AdminActionActor) error {
	adminInfo := map[string]any{
		"admin_id":       actor.UserID,
		"admin_username": actor.Username,
	}

	switch req.Mode {
	case "add":
		if req.Value <= 0 {
			return ErrUserQuotaChangeZero
		}
		if err := billingapp.AdjustWalletQuota(user.Id, -req.Value); err != nil {
			return err
		}
		auditapp.RecordLogWithAdminInfo(user.Id, auditschema.LogTypeManage,
			fmt.Sprintf("管理员增加用户额度 %s", logger.LogQuota(req.Value)), adminInfo)
		return nil
	case "subtract":
		if req.Value <= 0 {
			return ErrUserQuotaChangeZero
		}
		if err := billingapp.AdjustWalletQuota(user.Id, req.Value); err != nil {
			return err
		}
		auditapp.RecordLogWithAdminInfo(user.Id, auditschema.LogTypeManage,
			fmt.Sprintf("管理员减少用户额度 %s", logger.LogQuota(req.Value)), adminInfo)
		return nil
	case "override":
		if req.Value == 0 {
			return ErrUserQuotaChangeZero
		}
		oldQuota := user.Quota
		if err := billingapp.SetWalletQuota(user.Id, req.Value); err != nil {
			return err
		}
		auditapp.RecordLogWithAdminInfo(user.Id, auditschema.LogTypeManage,
			fmt.Sprintf("管理员覆盖用户额度从 %s 为 %s", logger.LogQuota(oldQuota), logger.LogQuota(req.Value)), adminInfo)
		return nil
	default:
		return ErrInvalidParams
	}
}
