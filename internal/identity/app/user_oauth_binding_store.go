package app

import (
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func listUserOAuthBindingsByUserID(userID int) ([]*identitydomain.UserOAuthBinding, error) {
	var bindings []*identitydomain.UserOAuthBinding
	err := platformdb.DB.Where("user_id = ?", userID).Find(&bindings).Error
	return bindings, err
}

func deleteUserOAuthBinding(userID int, providerID int) error {
	return platformdb.DB.Where("user_id = ? AND provider_id = ?", userID, providerID).Delete(&identitydomain.UserOAuthBinding{}).Error
}
