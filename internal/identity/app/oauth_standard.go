package app

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
	"strconv"
)

type OAuthCallbackResult struct {
	Action string
	User   *AuthenticatedSessionUser
}

type OAuthUserDeletedError struct{}

func (e *OAuthUserDeletedError) Error() string {
	return "user has been deleted"
}

type OAuthRegistrationDisabledError struct{}

func (e *OAuthRegistrationDisabledError) Error() string {
	return "registration is disabled"
}

type OAuthUnknownProviderError struct{}

func (e *OAuthUnknownProviderError) Error() string {
	return i18n.MsgOAuthUnknownProvider
}

type OAuthStateInvalidError struct{}

func (e *OAuthStateInvalidError) Error() string {
	return i18n.MsgOAuthStateInvalid
}

// GenerateOAuthState creates a CSRF state value for the upcoming OAuth round-trip.
func GenerateOAuthState() string {
	return platformruntime.GetRandomString(12)
}

// ResolveOAuthProvider returns the registered standard/custom OAuth provider by slug.
func ResolveOAuthProvider(providerName string) (oauth.Provider, error) {
	provider := oauth.GetProvider(providerName)
	if provider == nil {
		return nil, &OAuthUnknownProviderError{}
	}
	return provider, nil
}

// ValidateOAuthState verifies the state returned by the provider.
func ValidateOAuthState(expectedState string, actualState string) error {
	if actualState == "" || expectedState == "" || actualState != expectedState {
		return &OAuthStateInvalidError{}
	}
	return nil
}

// CompleteOAuthFlow completes login or bind for standard OAuth providers.
func CompleteOAuthFlow(
	c *gin.Context,
	provider oauth.Provider,
	code string,
	isBind bool,
	sessionUserID int,
) (*OAuthCallbackResult, error) {
	if !provider.IsEnabled() {
		return nil, ErrOAuthProviderDisabled
	}

	token, err := provider.ExchangeToken(c.Request.Context(), code, c)
	if err != nil {
		return nil, err
	}

	oauthUser, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		return nil, err
	}

	if isBind {
		if err := bindOAuthAccount(provider, oauthUser, sessionUserID); err != nil {
			return nil, err
		}
		return &OAuthCallbackResult{Action: "bind"}, nil
	}

	user, err := findOrCreateOAuthUser(provider, oauthUser)
	if err != nil {
		return nil, err
	}
	if user.Status != constant.UserStatusEnabled {
		return nil, ErrOAuthUserBanned
	}

	return &OAuthCallbackResult{
		Action: "login",
		User:   BuildAuthenticatedSessionUser(user),
	}, nil
}

var (
	ErrOAuthProviderDisabled = errors.New(i18n.MsgOAuthNotEnabled)
	ErrOAuthUserBanned       = errors.New(i18n.MsgOAuthUserBanned)
	ErrOAuthAlreadyBound     = errors.New(i18n.MsgOAuthAlreadyBound)
)

func bindOAuthAccount(provider oauth.Provider, oauthUser *oauth.OAuthUser, userID int) error {
	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		return ErrOAuthAlreadyBound
	}
	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && legacyID != "" && provider.IsUserIDTaken(legacyID) {
		return ErrOAuthAlreadyBound
	}

	user, err := identitystore.LoadUserByID(userID, true)
	if err != nil {
		return err
	}

	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		return identitystore.UpdateUserOAuthBinding(user.Id, genericProvider.GetProviderId(), oauthUser.ProviderUserID)
	}

	provider.SetProviderUserID(user, oauthUser.ProviderUserID)
	return identitystore.UpdateUser(user, false)
}

func findOrCreateOAuthUser(provider oauth.Provider, oauthUser *oauth.OAuthUser) (*identityschema.User, error) {
	user := &identityschema.User{}

	if provider.IsUserIDTaken(oauthUser.ProviderUserID) {
		if err := provider.FillUserByProviderID(user, oauthUser.ProviderUserID); err != nil {
			return nil, err
		}
		if user.Id == 0 {
			return nil, &OAuthUserDeletedError{}
		}
		return user, nil
	}

	if legacyID, ok := oauthUser.Extra["legacy_id"].(string); ok && legacyID != "" && provider.IsUserIDTaken(legacyID) {
		if err := provider.FillUserByProviderID(user, legacyID); err != nil {
			return nil, err
		}
		if user.Id != 0 {
			if err := identitystore.UpdateUserGitHubID(user.Id, oauthUser.ProviderUserID); err != nil {
				platformobservability.SysError(fmt.Sprintf("[OAuth] Failed to migrate user %d: %s", user.Id, err.Error()))
			}
			return user, nil
		}
	}

	if !platformconfig.RegisterEnabled {
		return nil, &OAuthRegistrationDisabledError{}
	}

	user.Username = provider.GetProviderPrefix() + strconv.Itoa(identitystore.LoadMaxUserID()+1)
	if oauthUser.Username != "" {
		if exists, err := identitystore.UserExistsOrDeleted(oauthUser.Username, ""); err == nil && !exists && len(oauthUser.Username) <= identityschema.UserNameMaxLength {
			user.Username = oauthUser.Username
		}
	}
	if oauthUser.DisplayName != "" {
		user.DisplayName = oauthUser.DisplayName
	} else if oauthUser.Username != "" {
		user.DisplayName = oauthUser.Username
	} else {
		user.DisplayName = provider.GetName() + " User"
	}
	if oauthUser.Email != "" {
		user.Email = oauthUser.Email
	}
	user.Role = constant.RoleCommonUser
	user.Status = constant.UserStatusEnabled

	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
			if err := identitystore.CreateUserWithTx(tx, user); err != nil {
				return err
			}
			binding := &identitydomain.UserOAuthBinding{
				UserId:         user.Id,
				ProviderId:     genericProvider.GetProviderId(),
				ProviderUserId: oauthUser.ProviderUserID,
			}
			return identitystore.CreateUserOAuthBindingWithTx(tx, binding)
		})
		if err != nil {
			return nil, err
		}
		finalizeOAuthUserRegistration(user)
		return user, nil
	}

	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		if err := identitystore.CreateUserWithTx(tx, user); err != nil {
			return err
		}
		provider.SetProviderUserID(user, oauthUser.ProviderUserID)
		return tx.Model(user).Updates(map[string]any{
			"github_id":   user.GitHubId,
			"discord_id":  user.DiscordId,
			"oidc_id":     user.OidcId,
			"linux_do_id": user.LinuxDOId,
			"wechat_id":   user.WeChatId,
			"telegram_id": user.TelegramId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	finalizeOAuthUserRegistration(user)
	return user, nil
}
