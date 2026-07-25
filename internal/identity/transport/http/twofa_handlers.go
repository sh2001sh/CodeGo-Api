package http

import (
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type twoFARequest struct {
	Code string `json:"code" binding:"required"`
}

func SetupTwoFA(c *gin.Context) {
	payload, err := identityapp.InitializeTwoFA(c.GetInt("id"))
	if err != nil {
		handleTwoFAError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "2FA设置初始化成功，请使用认证器扫描二维码并输入验证码完成设置",
		"data":    payload,
	})
}

func EnableTwoFA(c *gin.Context) {
	var req twoFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "参数错误")
		return
	}

	if err := identityapp.EnableTwoFA(c.GetInt("id"), req.Code); err != nil {
		handleTwoFAError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "两步验证启用成功",
	})
}

func DisableTwoFA(c *gin.Context) {
	var req twoFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "参数错误")
		return
	}

	if err := identityapp.DisableTwoFA(c.GetInt("id"), req.Code); err != nil {
		handleTwoFAError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "两步验证已禁用",
	})
}

func GetTwoFAStatus(c *gin.Context) {
	status, err := identityapp.LoadTwoFAStatus(c.GetInt("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, status)
}

func RegenerateBackupCodes(c *gin.Context) {
	var req twoFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "参数错误")
		return
	}

	backupCodes, err := identityapp.RegenerateTwoFABackupCodes(c.GetInt("id"), req.Code)
	if err != nil {
		handleTwoFAError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "备用码重新生成成功",
		"data": gin.H{
			"backup_codes": backupCodes,
		},
	})
}

func Verify2FALogin(c *gin.Context) {
	var req twoFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "参数错误")
		return
	}

	session := sessions.Default(c)
	pendingUserID := session.Get("pending_user_id")
	if pendingUserID == nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrSessionExpired.Error())
		return
	}
	userID, ok := pendingUserID.(int)
	if !ok {
		httpapi.ApiErrorMsg(c, identityapp.ErrSessionInvalid.Error())
		return
	}

	user, err := identityapp.VerifyTwoFALogin(userID, req.Code)
	if err != nil {
		handleTwoFAError(c, err)
		return
	}

	session.Delete(identityapp.PendingUsernameSessionKey)
	session.Delete(identityapp.PendingUserIDSessionKey)
	if err := establishAuthenticatedSession(c, session, identityapp.BuildAuthenticatedSessionUser(user)); err != nil {
		httpapi.ApiErrorMsg(c, err.Error())
		return
	}
}

func Admin2FAStats(c *gin.Context) {
	stats, err := identityapp.LoadTwoFAStats()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, stats)
}

func AdminDisable2FA(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, identityapp.ErrTwoFAUserIDFormat.Error())
		return
	}

	err = identityapp.ForceDisableTwoFA(userID, c.GetInt("id"), c.GetInt("role"), c.GetString("username"))
	if err != nil {
		if err == identitydomain.ErrTwoFANotEnabled {
			httpapi.ApiErrorMsg(c, "用户未启用2FA")
			return
		}
		handleTwoFAError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户2FA已被强制禁用",
	})
}

func handleTwoFAError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case identityapp.ErrTwoFAAlreadyEnabled,
		identityapp.ErrTwoFASetupMissing,
		identityapp.ErrTwoFAAlreadyActive,
		identityapp.ErrTwoFANotEnabled,
		identityapp.ErrTwoFAInvalidCode,
		identityapp.ErrTwoFASecretGenerationFailed,
		identityapp.ErrTwoFABackupCodeGenerationFailed,
		identityapp.ErrTwoFABackupCodeSaveFailed,
		identityapp.ErrSessionExpired,
		identityapp.ErrSessionInvalid,
		identityapp.ErrTwoFAUserNotFound,
		identityapp.ErrTwoFAUserIDFormat,
		identityapp.ErrTwoFANoPermission:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiError(c, err)
	}
}
