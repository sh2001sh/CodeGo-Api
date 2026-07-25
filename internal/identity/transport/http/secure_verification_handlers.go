package http

import (
	"fmt"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"

	"github.com/gin-gonic/gin"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/sessionstate"
)

type UniversalVerifyRequest struct {
	Method string `json:"method"`
	Code   string `json:"code,omitempty"`
}

func UniversalVerify(c *gin.Context) {
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未登录",
		})
		return
	}

	var req UniversalVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiError(c, fmt.Errorf("参数错误: %v", err))
		return
	}

	if _, err := identityapp.LoadActiveUser(userID); err != nil {
		httpapi.ApiError(c, fmt.Errorf("获取用户信息失败: %v", err))
		return
	}

	twoFA, err := identityapp.LoadEnabledTwoFA(userID)
	if err != nil {
		httpapi.ApiError(c, fmt.Errorf("获取安全验证信息失败: %v", err))
		return
	}
	has2FA := twoFA != nil
	hasPasskey, err := identityapp.HasPasskeyCredential(userID)
	if err != nil {
		httpapi.ApiError(c, fmt.Errorf("获取安全验证信息失败: %v", err))
		return
	}
	if !has2FA && !hasPasskey {
		httpapi.ApiError(c, fmt.Errorf("用户未启用2FA或Passkey"))
		return
	}

	var verified bool
	var verifyMethod string

	switch req.Method {
	case "2fa":
		if !has2FA {
			httpapi.ApiError(c, fmt.Errorf("用户未启用2FA"))
			return
		}
		if req.Code == "" {
			httpapi.ApiError(c, fmt.Errorf("验证码不能为空"))
			return
		}
		verified = identityapp.ValidateTwoFACodeForSecurityVerification(twoFA, req.Code)
		verifyMethod = "2FA"
	case "passkey":
		if !hasPasskey {
			httpapi.ApiError(c, fmt.Errorf("用户未启用Passkey"))
			return
		}
		var consumeErr error
		verified, consumeErr = sessionstate.ConsumePasskeyReady(c)
		if consumeErr != nil {
			httpapi.ApiError(c, fmt.Errorf("Passkey 验证状态异常: %v", consumeErr))
			return
		}
		if !verified {
			httpapi.ApiError(c, fmt.Errorf("请先完成 Passkey 验证"))
			return
		}
		verifyMethod = "Passkey"
	default:
		httpapi.ApiError(c, fmt.Errorf("不支持的验证方式: %s", req.Method))
		return
	}

	if !verified {
		httpapi.ApiError(c, fmt.Errorf("验证失败，请检查验证码"))
		return
	}

	now, saveErr := sessionstate.SetSecureVerificationSession(c, req.Method)
	if saveErr != nil {
		httpapi.ApiError(c, fmt.Errorf("保存验证状态失败: %v", saveErr))
		return
	}

	auditapp.RecordLog(userID, auditschema.LogTypeSystem, fmt.Sprintf("通用安全验证成功 (验证方式: %s)", verifyMethod))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "验证成功",
		"data": gin.H{
			"verified":   true,
			"expires_at": now + sessionstate.SecureVerificationTimeout,
		},
	})
}
