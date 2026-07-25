package app

import (
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func listCustomOAuthProviders() ([]*identitydomain.CustomOAuthProvider, error) {
	var providers []*identitydomain.CustomOAuthProvider
	err := platformdb.DB.Order("id asc").Find(&providers).Error
	return providers, err
}
