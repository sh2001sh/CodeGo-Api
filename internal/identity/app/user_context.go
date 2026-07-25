package app

import (
	"github.com/gin-gonic/gin"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

// LoadUserByID returns the current persisted user snapshot.
func LoadUserByID(userID int, selectAll bool) (*identityschema.User, error) {
	return identitystore.LoadUserByID(userID, selectAll)
}

// LoadUserCacheSnapshot returns the cached user projection used by auth flows.
func LoadUserCacheSnapshot(userID int) (*identityschema.UserBase, error) {
	return identitystore.LoadUserCacheSnapshot(userID)
}

// LoadUserGroup returns the user's current group label.
func LoadUserGroup(userID int, selectAll bool) (string, error) {
	return identitystore.LoadUserGroup(userID, selectAll)
}

// LoadUserFromAccessToken resolves a user from a management access token.
func LoadUserFromAccessToken(accessToken string) (*identityschema.User, error) {
	return identitystore.LoadUserFromAccessToken(accessToken)
}

// IsUserAdmin reports whether the current user has admin-or-higher privileges.
func IsUserAdmin(userID int) bool {
	return identitystore.IsUserAdmin(userID)
}

// WriteUserCacheToContext loads the user's cached profile snapshot and writes it into request context.
func WriteUserCacheToContext(c *gin.Context, userID int) error {
	return identitystore.WriteUserCacheToContext(c, userID)
}

// WriteUserContext writes an already loaded user cache snapshot into request context.
func WriteUserContext(c *gin.Context, user *identityschema.UserBase) {
	identitystore.WriteUserContext(c, user)
}

// LoadUserLanguage returns the user's language preference from the cached profile snapshot.
func LoadUserLanguage(userID int) string {
	return identitystore.LoadUserLanguage(userID)
}
