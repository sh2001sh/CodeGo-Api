package app

import (
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformerrx "github.com/sh2001sh/CodeGo-Api/internal/platform/errx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformvalidation "github.com/sh2001sh/CodeGo-Api/internal/platform/validation"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	DisplayName      string `json:"display_name"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
}

type AuthenticatedSessionUser struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Group       string `json:"group"`
}

type LoginResult struct {
	RequireTwoFA bool
	User         *AuthenticatedSessionUser
}

var (
	ErrPasswordLoginDisabled    = errors.New(i18n.MsgUserPasswordLoginDisabled)
	ErrRegisterDisabled         = errors.New(i18n.MsgUserRegisterDisabled)
	ErrPasswordRegisterDisabled = errors.New(i18n.MsgUserPasswordRegisterDisabled)
	ErrInvalidParams            = errors.New(i18n.MsgInvalidParams)
	ErrDatabaseError            = errors.New(i18n.MsgDatabaseError)
	ErrUsernameOrPasswordError  = errors.New(i18n.MsgUserUsernameOrPasswordError)
	ErrRequireTwoFA             = errors.New(i18n.MsgUserRequire2FA)
	ErrSessionSaveFailed        = errors.New(i18n.MsgUserSessionSaveFailed)
	ErrRegisterFailed           = errors.New(i18n.MsgUserRegisterFailed)
	ErrDefaultTokenFailed       = errors.New(i18n.MsgUserDefaultTokenFailed)
	ErrCreateDefaultToken       = errors.New(i18n.MsgCreateDefaultTokenErr)
	ErrEmailVerificationNeeded  = errors.New(i18n.MsgUserEmailVerificationRequired)
	ErrVerificationCodeInvalid  = errors.New(i18n.MsgUserVerificationCodeError)
	ErrUserExists               = errors.New(i18n.MsgUserExists)
)

// AuthenticatePasswordLogin validates a username/password login request.
func AuthenticatePasswordLogin(req LoginRequest) (*LoginResult, error) {
	if !platformconfig.PasswordLoginEnabled {
		return nil, ErrPasswordLoginDisabled
	}
	if req.Username == "" || req.Password == "" {
		return nil, ErrInvalidParams
	}

	user, err := identitystore.AuthenticateUserCredentials(req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, platformerrx.ErrDatabase):
			platformobservability.SysLog(fmt.Sprintf("Login database error for user %s: %v", req.Username, err))
			return nil, ErrDatabaseError
		case errors.Is(err, identitydomain.ErrUserEmptyCredentials):
			return nil, ErrInvalidParams
		default:
			return nil, ErrUsernameOrPasswordError
		}
	}

	result := &LoginResult{}
	result.User = BuildAuthenticatedSessionUser(user)
	if identitystore.IsTwoFAEnabled(user.Id) {
		result.RequireTwoFA = true
		return result, nil
	}
	return result, nil
}

// RegisterPasswordUser creates a new password-auth user and optional default token.
func RegisterPasswordUser(req RegisterRequest) error {
	if !platformconfig.RegisterEnabled {
		return ErrRegisterDisabled
	}
	if !platformconfig.PasswordRegisterEnabled {
		return ErrPasswordRegisterDisabled
	}

	user := identityschema.User{
		Username:         req.Username,
		Password:         req.Password,
		DisplayName:      req.DisplayName,
		Email:            req.Email,
		VerificationCode: req.VerificationCode,
	}
	if err := platformvalidation.Validate.Struct(&user); err != nil {
		return err
	}

	if platformconfig.EmailVerificationEnabled {
		if user.Email == "" || user.VerificationCode == "" {
			return ErrEmailVerificationNeeded
		}
		if !VerifyCodeWithKey(user.Email, user.VerificationCode, EmailVerificationPurpose) {
			return ErrVerificationCodeInvalid
		}
	}

	exist, err := identitystore.UserExistsOrDeleted(user.Username, user.Email)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("CheckUserExistOrDeleted error: %v", err))
		return ErrDatabaseError
	}
	if exist {
		return ErrUserExists
	}

	cleanUser := identityschema.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		Role:        constant.RoleCommonUser,
	}
	if platformconfig.EmailVerificationEnabled {
		cleanUser.Email = user.Email
	}
	if err := createUserAndRecordRegistration(&cleanUser); err != nil {
		return err
	}

	if !constant.GenerateDefaultToken {
		return nil
	}

	if cleanUser.Id <= 0 {
		return ErrRegisterFailed
	}
	key, err := platformruntime.GenerateKey()
	if err != nil {
		platformobservability.SysLog("failed to generate token key: " + err.Error())
		return ErrDefaultTokenFailed
	}

	token := identityschema.Token{
		UserId:             cleanUser.Id,
		Name:               cleanUser.Username + "的初始令牌",
		Key:                key,
		CreatedTime:        platformruntime.GetTimestamp(),
		AccessedTime:       platformruntime.GetTimestamp(),
		ExpiredTime:        -1,
		RemainQuota:        500000,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
	}
	if gatewaygroups.DefaultUseAutoGroup {
		token.Group = "auto"
	}
	if err := InsertUserToken(&token); err != nil {
		return ErrCreateDefaultToken
	}
	return nil
}

// BuildAuthenticatedSessionUser returns the safe session payload for a logged-in user.
func BuildAuthenticatedSessionUser(user *identityschema.User) *AuthenticatedSessionUser {
	if user == nil {
		return nil
	}
	return &AuthenticatedSessionUser{
		ID:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Group:       user.Group,
	}
}
