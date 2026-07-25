package app

import (
	"encoding/json"
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformvalidation "github.com/sh2001sh/CodeGo-Api/internal/platform/validation"
)

var (
	ErrInvalidInput         = errors.New(i18n.MsgInvalidInput)
	ErrGenerateAccessFailed = errors.New(i18n.MsgGenerateFailed)
	ErrUUIDDuplicate        = errors.New(i18n.MsgUuidDuplicate)
	ErrCannotDeleteRootUser = errors.New(i18n.MsgUserCannotDeleteRootUser)
	ErrOriginalPassword     = errors.New("原密码错误")
)

// UpdateSelfProfileRequest captures the writable self-service profile fields.
type UpdateSelfProfileRequest struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	OriginalPassword string `json:"original_password"`
	DisplayName      string `json:"display_name"`
}

// UpdateSelfSidebarModules persists the authenticated user's sidebar module layout.
func UpdateSelfSidebarModules(userID int, rawValue any) error {
	user, err := LoadUserByID(userID, false)
	if err != nil {
		return err
	}

	currentSetting := identitydomain.GetSetting(user)
	if sidebarModules, ok := rawValue.(string); ok {
		currentSetting.SidebarModules = sidebarModules
	}

	identitydomain.SetSetting(user, currentSetting)
	if err := identitystore.UpdateUser(user, false); err != nil {
		return ErrUpdateFailed
	}
	return nil
}

// UpdateSelfLanguage persists the authenticated user's interface language preference.
func UpdateSelfLanguage(userID int, rawValue any) error {
	user, err := LoadUserByID(userID, false)
	if err != nil {
		return err
	}

	currentSetting := identitydomain.GetSetting(user)
	if language, ok := rawValue.(string); ok {
		currentSetting.Language = language
	}

	identitydomain.SetSetting(user, currentSetting)
	if err := identitystore.UpdateUser(user, false); err != nil {
		return ErrUpdateFailed
	}
	return nil
}

// DecodeUpdateSelfProfileRequest converts the legacy loose request payload into a typed self-service request.
func DecodeUpdateSelfProfileRequest(requestData map[string]any) (UpdateSelfProfileRequest, error) {
	payload, err := json.Marshal(requestData)
	if err != nil {
		return UpdateSelfProfileRequest{}, ErrInvalidParams
	}

	var req UpdateSelfProfileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return UpdateSelfProfileRequest{}, ErrInvalidParams
	}
	return req, nil
}

// UpdateSelfProfile updates the authenticated user's own profile and password.
func UpdateSelfProfile(userID int, req UpdateSelfProfileRequest) error {
	candidate := identityschema.User{
		Username:         req.Username,
		Password:         req.Password,
		OriginalPassword: req.OriginalPassword,
		DisplayName:      req.DisplayName,
	}

	if candidate.Password == "" {
		candidate.Password = "$I_LOVE_U"
	}
	if err := platformvalidation.Validate.Struct(&candidate); err != nil {
		return ErrInvalidInput
	}

	cleanUser := identityschema.User{
		Id:          userID,
		Username:    candidate.Username,
		Password:    candidate.Password,
		DisplayName: candidate.DisplayName,
	}
	if candidate.Password == "$I_LOVE_U" {
		cleanUser.Password = ""
	}

	updatePassword, err := checkSelfPasswordUpdate(req.OriginalPassword, cleanUser.Password, userID)
	if err != nil {
		return err
	}
	return identitystore.UpdateUser(&cleanUser, updatePassword)
}

// DeleteSelf soft-deletes the authenticated user's own account unless it is the root account.
func DeleteSelf(userID int) error {
	user, err := LoadUserByID(userID, false)
	if err != nil {
		return err
	}
	if user.Role == constant.RoleRootUser {
		return ErrCannotDeleteRootUser
	}
	return identitystore.DeleteUserByID(userID)
}

// GenerateAccessToken rotates the authenticated user's system access token.
func GenerateAccessToken(userID int) (string, error) {
	user, err := LoadUserByID(userID, true)
	if err != nil {
		return "", err
	}

	randI := platformruntime.GetRandomInt(4)
	key, err := platformruntime.GenerateRandomKey(29 + randI)
	if err != nil {
		platformobservability.SysLog("failed to generate key: " + err.Error())
		return "", ErrGenerateAccessFailed
	}
	user.SetAccessToken(key)

	var duplicate identityschema.User
	if platformdb.DB.Where("access_token = ?", user.AccessToken).First(&duplicate).RowsAffected != 0 {
		return "", ErrUUIDDuplicate
	}

	if err := identitystore.UpdateUser(user, false); err != nil {
		return "", err
	}
	return user.GetAccessToken(), nil
}

func checkSelfPasswordUpdate(originalPassword string, newPassword string, userID int) (bool, error) {
	currentUser, err := LoadUserByID(userID, true)
	if err != nil {
		return false, err
	}
	if !platformsecurity.ValidatePasswordAndHash(originalPassword, currentUser.Password) && currentUser.Password != "" {
		return false, ErrOriginalPassword
	}
	if newPassword == "" {
		return false, nil
	}
	return true, nil
}
