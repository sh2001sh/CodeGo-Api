package http

import (
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
)

type codexOAuthCompleteRequest struct {
	Input string `json:"input"`
}

func StartCodexOAuth(c *gin.Context) {
	startCodexOAuthWithChannelID(c, 0)
}

func StartCodexOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	startCodexOAuthWithChannelID(c, channelID)
}

func startCodexOAuthWithChannelID(c *gin.Context, channelID int) {
	result, err := gatewayexecutionapp.StartCodexOAuth(channelID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "state"), result.State)
	session.Set(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "verifier"), result.Verifier)
	session.Set(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "created_at"), time.Now().Unix())
	_ = session.Save()

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"authorize_url": result.AuthorizeURL,
		},
	})
}

func CompleteCodexOAuth(c *gin.Context) {
	completeCodexOAuthWithChannelID(c, 0)
}

func CompleteCodexOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	completeCodexOAuthWithChannelID(c, channelID)
}

func completeCodexOAuthWithChannelID(c *gin.Context, channelID int) {
	var req codexOAuthCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiError(c, err)
		return
	}

	code, state, err := gatewayexecutionapp.ParseCodexAuthorizationInput(req.Input)
	if err != nil {
		platformobservability.SysError("failed to parse codex authorization input: " + err.Error())
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "解析授权信息失败，请检查输入格式"})
		return
	}
	if strings.TrimSpace(code) == "" {
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "missing authorization code"})
		return
	}
	if strings.TrimSpace(state) == "" {
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "missing state in input"})
		return
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "state")).(string)
	verifier, _ := session.Get(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "verifier")).(string)
	if strings.TrimSpace(expectedState) == "" || strings.TrimSpace(verifier) == "" {
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "oauth flow not started or session expired"})
		return
	}
	if state != expectedState {
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "state mismatch"})
		return
	}

	proxyURL := ""
	if channelID > 0 {
		channel, channelErr := gatewayexecutionapp.GetCodexChannelForOAuth(channelID)
		if channelErr != nil {
			httpapi.ApiError(c, channelErr)
			return
		}
		proxyURL = gatewaydomain.GetSettings(channel).Proxy
	}

	result, completeErr := gatewayexecutionapp.CompleteCodexOAuth(channelID, code, verifier, proxyURL)
	if completeErr != nil {
		platformobservability.SysError("failed to complete codex oauth: " + completeErr.Error())
		c.JSON(stdhttp.StatusOK, gin.H{"success": false, "message": "授权码交换失败，请重试"})
		return
	}

	session.Delete(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "state"))
	session.Delete(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "verifier"))
	session.Delete(gatewayexecutionapp.CodexOAuthSessionKey(channelID, "created_at"))
	_ = session.Save()

	message := "generated"
	if channelID > 0 {
		message = "saved"
	}

	data := gin.H{
		"account_id":   result.AccountID,
		"email":        result.Email,
		"expires_at":   result.ExpiresAt,
		"last_refresh": result.LastRefresh,
	}
	if result.Key != "" {
		data["key"] = result.Key
	}
	if result.ChannelID > 0 {
		data["channel_id"] = result.ChannelID
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func GetCodexChannelUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	result, err := gatewayexecutionapp.GetCodexChannelUsage(channelID)
	if err != nil {
		platformobservability.SysError("failed to fetch codex usage: " + err.Error())
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	message := ""
	if !result.Success {
		message = "upstream status: " + strconv.Itoa(result.UpstreamStatus)
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success":         result.Success,
		"message":         message,
		"upstream_status": result.UpstreamStatus,
		"data":            result.Data,
	})
}
