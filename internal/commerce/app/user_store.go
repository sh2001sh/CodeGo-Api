package app

import (
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

func loadCommerceUserByID(userID int, selectAll bool) (*identityschema.User, error) {
	return identitystore.LoadUserByID(userID, selectAll)
}

func loadCommerceUserGroup(userID int, selectAll bool) (string, error) {
	return identitystore.LoadUserGroup(userID, selectAll)
}
