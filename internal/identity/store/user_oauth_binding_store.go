package store

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"time"

	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

// LoadUserByOAuthBinding finds a user by provider ID and provider user ID.
func LoadUserByOAuthBinding(providerID int, providerUserID string) (*identityschema.User, error) {
	var binding identitydomain.UserOAuthBinding
	err := platformdb.DB.Where("provider_id = ? AND provider_user_id = ?", providerID, providerUserID).First(&binding).Error
	if err != nil {
		return nil, err
	}

	var user identityschema.User
	err = platformdb.DB.First(&user, binding.UserId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsOAuthProviderUserIDTaken reports whether a generic OAuth account is already bound.
func IsOAuthProviderUserIDTaken(providerID int, providerUserID string) bool {
	var count int64
	platformdb.DB.Model(&identitydomain.UserOAuthBinding{}).Where("provider_id = ? AND provider_user_id = ?", providerID, providerUserID).Count(&count)
	return count > 0
}

// IsGitHubIDTaken reports whether the GitHub user ID is already bound.
func IsGitHubIDTaken(githubID string) bool {
	return platformdb.DB.Unscoped().Where("github_id = ?", githubID).Find(&identityschema.User{}).RowsAffected == 1
}

// IsDiscordIDTaken reports whether the Discord user ID is already bound.
func IsDiscordIDTaken(discordID string) bool {
	return platformdb.DB.Unscoped().Where("discord_id = ?", discordID).Find(&identityschema.User{}).RowsAffected == 1
}

// IsOIDCIDTaken reports whether the OIDC subject is already bound.
func IsOIDCIDTaken(oidcID string) bool {
	return platformdb.DB.Where("oidc_id = ?", oidcID).Find(&identityschema.User{}).RowsAffected == 1
}

// IsLinuxDOIDTaken reports whether the LinuxDO user ID is already bound.
func IsLinuxDOIDTaken(linuxDOID string) bool {
	var user identityschema.User
	err := platformdb.DB.Unscoped().Where("linux_do_id = ?", linuxDOID).First(&user).Error
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

// CreateUserOAuthBindingWithTx creates a new OAuth binding within a transaction.
func CreateUserOAuthBindingWithTx(tx *gorm.DB, binding *identitydomain.UserOAuthBinding) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if binding == nil {
		return errors.New("binding is required")
	}
	if binding.UserId == 0 {
		return errors.New("user ID is required")
	}
	if binding.ProviderId == 0 {
		return errors.New("provider ID is required")
	}
	if binding.ProviderUserId == "" {
		return errors.New("provider user ID is required")
	}

	var count int64
	tx.Model(&identitydomain.UserOAuthBinding{}).
		Where("provider_id = ? AND provider_user_id = ?", binding.ProviderId, binding.ProviderUserId).
		Count(&count)
	if count > 0 {
		return errors.New("this OAuth account is already bound to another user")
	}

	binding.CreatedAt = time.Now()
	return tx.Create(binding).Error
}

// UpdateUserOAuthBinding creates or updates one OAuth binding for a user.
func UpdateUserOAuthBinding(userID int, providerID int, newProviderUserID string) error {
	var existingBinding identitydomain.UserOAuthBinding
	err := platformdb.DB.Where("provider_id = ? AND provider_user_id = ?", providerID, newProviderUserID).First(&existingBinding).Error
	if err == nil && existingBinding.UserId != userID {
		return errors.New("this OAuth account is already bound to another user")
	}

	var binding identitydomain.UserOAuthBinding
	err = platformdb.DB.Where("user_id = ? AND provider_id = ?", userID, providerID).First(&binding).Error
	if err != nil {
		return createUserOAuthBinding(&identitydomain.UserOAuthBinding{
			UserId:         userID,
			ProviderId:     providerID,
			ProviderUserId: newProviderUserID,
		})
	}

	return platformdb.DB.Model(&binding).Update("provider_user_id", newProviderUserID).Error
}

func createUserOAuthBinding(binding *identitydomain.UserOAuthBinding) error {
	if binding == nil {
		return errors.New("binding is required")
	}
	if binding.UserId == 0 {
		return errors.New("user ID is required")
	}
	if binding.ProviderId == 0 {
		return errors.New("provider ID is required")
	}
	if binding.ProviderUserId == "" {
		return errors.New("provider user ID is required")
	}
	if IsOAuthProviderUserIDTaken(binding.ProviderId, binding.ProviderUserId) {
		return errors.New("this OAuth account is already bound to another user")
	}

	binding.CreatedAt = time.Now()
	return platformdb.DB.Create(binding).Error
}
