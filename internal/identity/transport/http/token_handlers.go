package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"
	"strings"
)

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func GetAllTokens(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := platformpagination.GetPageQuery(c)
	tokens, total, err := identityapp.ListUserTokens(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(identityapp.BuildMaskedTokenResponses(tokens))
	httpapi.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userID := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := platformpagination.GetPageQuery(c)

	tokens, total, err := identityapp.SearchUserTokens(userID, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(identityapp.BuildMaskedTokenResponses(tokens))
	httpapi.ApiSuccess(c, pageInfo)
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userID := c.GetInt("id")
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	token, err := identityapp.GetUserToken(userID, id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, identityapp.BuildMaskedTokenResponse(token))
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userID := c.GetInt("id")
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	token, err := identityapp.GetUserToken(userID, id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenID := c.GetInt("token_id")
	userID := c.GetInt("id")
	token, err := identityapp.GetUserToken(userID, tokenID)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0,
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := identityapp.GetTokenByBearerKey(tokenKey)
	if err != nil {
		platformobservability.SysError("failed to get token by key: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	token := identityschema.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		httpapi.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := int((1000000000 * platformruntime.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	maxTokens := identitystore.GetMaxUserTokens()
	count, err := identityapp.CountTokensForUser(c.GetInt("id"))
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	key, err := platformruntime.GenerateKey()
	if err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		platformobservability.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := identityschema.Token{
		UserId:             c.GetInt("id"),
		Name:               token.Name,
		Key:                key,
		CreatedTime:        platformruntime.GetTimestamp(),
		AccessedTime:       platformruntime.GetTimestamp(),
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		AllowIps:           token.AllowIps,
		Group:              gatewayroutingapp.NormalizeTokenGroup(token.Group),
		CrossGroupRetry:    token.CrossGroupRetry,
	}
	err = identityapp.InsertUserToken(&cleanToken)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID := c.GetInt("id")
	err := identityapp.DeleteUserToken(userID, id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userID := c.GetInt("id")
	statusOnly := c.Query("status_only")
	token := identityschema.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		httpapi.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := int((1000000000 * platformruntime.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	cleanToken, err := identityapp.GetUserToken(userID, token.Id)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	if token.Status == constant.TokenStatusEnabled {
		if cleanToken.Status == constant.TokenStatusExpired && cleanToken.ExpiredTime <= platformruntime.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if cleanToken.Status == constant.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			httpapi.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = gatewayroutingapp.NormalizeTokenGroup(token.Group)
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
	}
	err = identityapp.UpdateUserToken(cleanToken)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    identityapp.BuildMaskedTokenResponse(cleanToken),
	})
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userID := c.GetInt("id")
	count, err := identityapp.BatchDeleteUserTokens(userID, tokenBatch.Ids)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		httpapi.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userID := c.GetInt("id")
	keysMap, err := identityapp.GetUserTokenKeys(userID, tokenBatch.Ids)
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, gin.H{"keys": keysMap})
}
