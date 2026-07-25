package store

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
)

func WriteUserContext(c *gin.Context, user *identityschema.UserBase) {
	if c == nil || user == nil {
		return
	}
	httpctx.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	httpctx.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	httpctx.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	httpctx.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	httpctx.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	httpctx.SetContextKey(c, constant.ContextKeyUserSetting, identitydomain.GetBaseSetting(user))
}
