package sessionstate

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

// SaveAuthenticatedSession persists the authenticated user session fields after a successful login.
func SaveAuthenticatedSession(c *gin.Context, user *identityschema.User) error {
	if user == nil {
		return nil
	}
	identitystore.TouchUserLastLoginAt(user.Id)
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	return session.Save()
}
