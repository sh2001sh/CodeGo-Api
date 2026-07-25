package middleware

import (
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformerrx "github.com/sh2001sh/CodeGo-Api/internal/platform/errx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"gorm.io/gorm"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func validUserInfo(username string, role int) bool {
	// check username is empty
	if strings.TrimSpace(username) == "" {
		return false
	}
	if !constant.IsValidateRole(role) {
		return false
	}
	return true
}

func authHelper(c *gin.Context, minRole int) {
	session := sessions.Default(c)
	username := session.Get("username")
	role := session.Get("role")
	id := session.Get("id")
	status := session.Get("status")
	group := session.Get("group")
	useAccessToken := false
	currentUsername, _ := username.(string)
	currentRole, _ := role.(int)
	currentUserID, _ := id.(int)
	currentStatus, _ := status.(int)
	currentGroup, _ := group.(string)
	if username == nil {
		// Check access token
		accessToken := c.Request.Header.Get("Authorization")
		if accessToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgAuthNotLoggedIn),
			})
			c.Abort()
			return
		}
		user, authErr := identityapp.LoadUserFromAccessToken(accessToken)
		if authErr != nil {
			if errors.Is(authErr, platformerrx.ErrDatabase) {
				platformobservability.SysLog("ValidateAccessToken database error: " + authErr.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgDatabaseError),
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgAuthAccessTokenInvalid),
				})
			}
			c.Abort()
			return
		}
		if user != nil && user.Username != "" {
			if !validUserInfo(user.Username, user.Role) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserInfoInvalid),
				})
				c.Abort()
				return
			}
			// Token is valid
			username = user.Username
			role = user.Role
			id = user.Id
			status = user.Status
			group = user.Group
			currentUsername = user.Username
			currentRole = user.Role
			currentUserID = user.Id
			currentStatus = user.Status
			currentGroup = user.Group
			useAccessToken = true
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgAuthAccessTokenInvalid),
			})
			c.Abort()
			return
		}
	} else if platformdb.DB != nil && currentUserID > 0 {
		// Refresh session-backed user state so promote/demote/disable changes
		// take effect immediately without requiring a new login.
		user, err := identityapp.LoadUserByID(currentUserID, false)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserInfoInvalid),
				})
			} else {
				platformobservability.SysLog("authHelper refresh user info database error: " + err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgDatabaseError),
				})
			}
			c.Abort()
			return
		}
		if !validUserInfo(user.Username, user.Role) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserInfoInvalid),
			})
			c.Abort()
			return
		}
		username = user.Username
		role = user.Role
		status = user.Status
		group = user.Group
		currentUsername = user.Username
		currentRole = user.Role
		currentStatus = user.Status
		currentGroup = user.Group
		if session.Get("username") != user.Username ||
			session.Get("role") != user.Role ||
			session.Get("status") != user.Status ||
			session.Get("group") != user.Group {
			session.Set("username", user.Username)
			session.Set("role", user.Role)
			session.Set("status", user.Status)
			session.Set("group", user.Group)
			if err := session.Save(); err != nil {
				platformobservability.SysLog("authHelper failed to sync session user state: " + err.Error())
			}
		}
	}
	// get header CodeGo-Api-User
	apiUserIdStr := c.Request.Header.Get("CodeGo-Api-User")
	if apiUserIdStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserIdNotProvided),
		})
		c.Abort()
		return
	}
	apiUserId, err := strconv.Atoi(apiUserIdStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserIdFormatError),
		})
		c.Abort()
		return

	}
	if currentUserID != apiUserId {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserIdMismatch),
		})
		c.Abort()
		return
	}
	if currentStatus == constant.UserStatusDisabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserBanned),
		})
		c.Abort()
		return
	}
	if currentRole < minRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
		})
		c.Abort()
		return
	}
	if !validUserInfo(currentUsername, currentRole) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserInfoInvalid),
		})
		c.Abort()
		return
	}
	// 防止不同codegoapi版本冲突，导致数据不通用
	c.Header("Auth-Version", "864b7076dbcd0a3c01b5520316720ebf")
	c.Set("username", currentUsername)
	c.Set("role", currentRole)
	c.Set("id", currentUserID)
	c.Set("group", currentGroup)
	c.Set("user_group", currentGroup)
	c.Set("use_access_token", useAccessToken)
	if !enforceGlobalAuthenticatedAPIRateLimit(c) {
		return
	}

	c.Next()
}

func TryUserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id := session.Get("id")
		if id != nil {
			c.Set("id", id)
		}
		c.Next()
	}
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, constant.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, constant.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, constant.RoleRootUser)
	}
}

func WssAuth(c *gin.Context) {

}

// TokenOrUserAuth allows either session-based user auth or API token auth.
// Used for endpoints that need to be accessible from both the dashboard and API clients.
func TokenOrUserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// Try session auth first (dashboard users)
		session := sessions.Default(c)
		if id := session.Get("id"); id != nil {
			if status, ok := session.Get("status").(int); ok && status == constant.UserStatusEnabled {
				c.Set("id", id)
				c.Next()
				return
			}
		}
		// Fall back to token auth (API clients)
		TokenAuth()(c)
	}
}

// TokenAuthReadOnly 宽松版本的令牌认证中间件，用于只读查询接口。
// 只验证令牌 key 是否存在，不检查令牌状态、过期时间和额度。
// 即使令牌已过期、已耗尽或已禁用，也允许访问。
// 仍然检查用户是否被封禁。
func TokenAuthReadOnly() func(c *gin.Context) {
	return func(c *gin.Context) {
		key := c.Request.Header.Get("Authorization")
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgTokenNotProvided),
			})
			c.Abort()
			return
		}
		if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		key = strings.TrimPrefix(key, "sk-")
		parts := strings.Split(key, "-")
		key = parts[0]

		token, err := identityapp.GetTokenByBearerKey(key)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgTokenInvalid),
				})
			} else {
				platformobservability.SysLog("TokenAuthReadOnly GetTokenByKey database error: " + err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": httpapi.TranslateMessage(c, i18n.MsgDatabaseError),
				})
			}
			c.Abort()
			return
		}

		userCache, err := identityapp.LoadUserCacheSnapshot(token.UserId)
		if err != nil {
			platformobservability.SysLog(fmt.Sprintf("TokenAuthReadOnly GetUserCache error for user %d: %v", token.UserId, err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgDatabaseError),
			})
			c.Abort()
			return
		}
		if userCache.Status != constant.UserStatusEnabled {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": httpapi.TranslateMessage(c, i18n.MsgAuthUserBanned),
			})
			c.Abort()
			return
		}

		c.Set("id", token.UserId)
		c.Set("token_id", token.Id)
		c.Set("token_key", token.Key)
		c.Next()
	}
}

func TokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 先检测是否为ws
		if c.Request.Header.Get("Sec-WebSocket-Protocol") != "" {
			// Sec-WebSocket-Protocol: realtime, openai-insecure-api-key.sk-xxx, openai-beta.realtime-v1
			// read sk from Sec-WebSocket-Protocol
			key := c.Request.Header.Get("Sec-WebSocket-Protocol")
			parts := strings.Split(key, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "openai-insecure-api-key") {
					key = strings.TrimPrefix(part, "openai-insecure-api-key.")
					break
				}
			}
			c.Request.Header.Set("Authorization", "Bearer "+key)
		}
		// 检查path包含/v1/messages 或 /v1/models
		if strings.Contains(c.Request.URL.Path, "/v1/messages") || strings.Contains(c.Request.URL.Path, "/v1/models") {
			anthropicKey := c.Request.Header.Get("x-api-key")
			if anthropicKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+anthropicKey)
			}
		}
		// gemini api 从query中获取key
		if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1beta/openai/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
			skKey := c.Query("key")
			if skKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+skKey)
			}
			// 从x-goog-api-key header中获取key
			xGoogKey := c.Request.Header.Get("x-goog-api-key")
			if xGoogKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+xGoogKey)
			}
		}
		key := c.Request.Header.Get("Authorization")
		token, parts, err := identityapp.ValidateUserBearerToken(key)
		if token != nil {
			id := c.GetInt("id")
			if id == 0 {
				c.Set("id", token.UserId)
			}
		}
		if err != nil {
			if errors.Is(err, platformerrx.ErrDatabase) {
				platformobservability.SysLog("TokenAuth ValidateUserToken database error: " + err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError,
					httpapi.TranslateMessage(c, i18n.MsgDatabaseError))
			} else {
				abortWithOpenAiMessage(c, http.StatusUnauthorized,
					httpapi.TranslateMessage(c, i18n.MsgTokenInvalid))
			}
			return
		}

		allowIps := token.GetIpLimits()
		if len(allowIps) > 0 {
			clientIp := c.ClientIP()
			logger.LogDebug(c, "Token has IP restrictions, checking client IP %s", clientIp)
			ip := net.ParseIP(clientIp)
			if ip == nil {
				abortWithOpenAiMessage(c, http.StatusForbidden, "无法解析客户端 IP 地址")
				return
			}
			if platformsecurity.IsIPInCIDRList(ip, allowIps) == false {
				abortWithOpenAiMessage(c, http.StatusForbidden, "您的 IP 不在令牌允许访问的列表中", types.ErrorCodeAccessDenied)
				return
			}
			logger.LogDebug(c, "Client IP %s passed the token IP restrictions check", clientIp)
		}

		userCache, err := identityapp.LoadUserCacheSnapshot(token.UserId)
		if err != nil {
			platformobservability.SysLog(fmt.Sprintf("TokenAuth GetUserCache error for user %d: %v", token.UserId, err))
			abortWithOpenAiMessage(c, http.StatusInternalServerError,
				httpapi.TranslateMessage(c, i18n.MsgDatabaseError))
			return
		}
		userEnabled := userCache.Status == constant.UserStatusEnabled
		if !userEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, httpapi.TranslateMessage(c, i18n.MsgAuthUserBanned))
			return
		}

		identityapp.WriteUserContext(c, userCache)

		userGroup := userCache.Group
		tokenGroup := token.Group
		if tokenGroup != "" {
			// check common.UserUsableGroups[userGroup]
			if _, ok := gatewayroutingapp.GetUserUsableGroups(userGroup)[tokenGroup]; !ok {
				abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("无权访问 %s 分组", tokenGroup))
				return
			}
			// check group in common.GroupRatio
			if !gatewaystore.ContainsGroupRatio(tokenGroup) {
				if tokenGroup != "auto" {
					abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 已被弃用", tokenGroup))
					return
				}
			}
			userGroup = tokenGroup
		}
		httpctx.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)

		err = SetupContextForToken(c, token, parts...)
		if err != nil {
			return
		}
		c.Next()
	}
}

func SetupContextForToken(c *gin.Context, token *identityschema.Token, parts ...string) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	c.Set("id", token.UserId)
	c.Set("token_id", token.Id)
	c.Set("token_key", token.Key)
	c.Set("token_name", token.Name)
	c.Set("token_unlimited_quota", token.UnlimitedQuota)
	if !token.UnlimitedQuota {
		c.Set("token_quota", token.RemainQuota)
	}
	if token.ModelLimitsEnabled {
		c.Set("token_model_limit_enabled", true)
		c.Set("token_model_limit", token.GetModelLimitsMap())
	} else {
		c.Set("token_model_limit_enabled", false)
	}
	httpctx.SetContextKey(c, constant.ContextKeyTokenGroup, gatewayroutingapp.NormalizeTokenGroup(token.Group))
	httpctx.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, token.CrossGroupRetry)
	if len(parts) > 1 {
		if identityapp.IsUserAdmin(token.UserId) {
			c.Set("specific_channel_id", parts[1])
		} else {
			c.Header("specific_channel_version", "701e3ae1dc3f7975556d354e0675168d004891c8")
			abortWithOpenAiMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
			return fmt.Errorf("普通用户不支持指定渠道")
		}
	}
	return nil
}
