package app

import (
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
)

var (
	ErrCustomOAuthNotLoggedIn  = errors.New("未登录")
	ErrCustomOAuthNoPermission = errors.New("no permission")
)

// UserOAuthBindingView is the transport payload for a user's custom OAuth bindings.
type UserOAuthBindingView struct {
	ProviderID     int    `json:"provider_id"`
	ProviderName   string `json:"provider_name"`
	ProviderSlug   string `json:"provider_slug"`
	ProviderIcon   string `json:"provider_icon"`
	ProviderUserID string `json:"provider_user_id"`
}

// ListUserOAuthBindings returns the current user's custom OAuth bindings.
func ListUserOAuthBindings(userID int) ([]UserOAuthBindingView, error) {
	if userID <= 0 {
		return nil, ErrCustomOAuthNotLoggedIn
	}

	bindings, err := listUserOAuthBindingsByUserID(userID)
	if err != nil {
		return nil, err
	}
	providers, err := listCustomOAuthProviders()
	if err != nil {
		return nil, err
	}

	providerByID := make(map[int]*identitydomain.CustomOAuthProvider, len(providers))
	for _, provider := range providers {
		providerByID[provider.Id] = provider
	}

	response := make([]UserOAuthBindingView, 0, len(bindings))
	for _, binding := range bindings {
		provider := providerByID[binding.ProviderId]
		if provider == nil {
			continue
		}
		response = append(response, UserOAuthBindingView{
			ProviderID:     binding.ProviderId,
			ProviderName:   provider.Name,
			ProviderSlug:   provider.Slug,
			ProviderIcon:   provider.Icon,
			ProviderUserID: binding.ProviderUserId,
		})
	}
	return response, nil
}

// ListUserOAuthBindingsAsAdmin returns a target user's bindings after role checks.
func ListUserOAuthBindingsAsAdmin(targetUserID int, actorRole int) ([]UserOAuthBindingView, error) {
	if err := authorizeManagedUser(targetUserID, actorRole); err != nil {
		return nil, err
	}
	return ListUserOAuthBindings(targetUserID)
}

// UnbindUserOAuth removes the current user's binding for a custom OAuth provider.
func UnbindUserOAuth(userID int, providerID int) error {
	if userID <= 0 {
		return ErrCustomOAuthNotLoggedIn
	}
	return deleteUserOAuthBinding(userID, providerID)
}

// UnbindUserOAuthAsAdmin removes a target user's binding after role checks.
func UnbindUserOAuthAsAdmin(targetUserID int, providerID int, actorRole int) error {
	if err := authorizeManagedUser(targetUserID, actorRole); err != nil {
		return err
	}
	return deleteUserOAuthBinding(targetUserID, providerID)
}

func authorizeManagedUser(targetUserID int, actorRole int) error {
	targetUser, err := LoadUserByID(targetUserID, false)
	if err != nil {
		return err
	}
	if actorRole <= targetUser.Role && actorRole != constant.RoleRootUser {
		return ErrCustomOAuthNoPermission
	}
	return nil
}
