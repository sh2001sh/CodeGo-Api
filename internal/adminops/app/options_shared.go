package app

import (
	"encoding/json"
	"errors"
	"fmt"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"strconv"
	"strings"
)

var (
	ErrOptionInvalidPayload            = errors.New("无效的参数")
	ErrOptionComplianceFieldImmutable  = errors.New("合规确认字段不允许通过通用设置接口修改")
	ErrOptionPaymentComplianceRequired = errors.New("payment compliance required")
	ErrOptionGitHubOAuthMissingConfig  = errors.New("无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！")
	ErrOptionDiscordOAuthMissingConfig = errors.New("无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！")
	ErrOptionOIDCMissingConfig         = errors.New("无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！")
	ErrOptionLinuxDOMissingConfig      = errors.New("无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！")
	ErrOptionEmailDomainMissingConfig  = errors.New("无法启用邮箱域名限制，请先填入限制的邮箱域名！")
	ErrOptionWeChatMissingConfig       = errors.New("无法启用微信登录，请先填入微信登录相关配置信息！")
	ErrOptionTurnstileMissingConfig    = errors.New("无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！")
	ErrOptionTelegramMissingConfig     = errors.New("无法启用 Telegram OAuth，请先填入 Telegram Bot Token！")
	ErrOptionThemeFrontendInvalid      = errors.New("无效的主题值，可选值：default（新版前端）")
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

// AdminOptionUpdateInput carries a generic option update request.
type AdminOptionUpdateInput struct {
	Key   string
	Value any
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := platformencoding.UnmarshalString(raw, &parsed); err != nil {
		return
	}
	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]gatewaystore.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = gatewaystore.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := platformencoding.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func normalizeOptionValue(value any) string {
	switch typed := value.(type) {
	case bool:
		return platformencoding.Interface2String(typed)
	case float64:
		return platformencoding.Interface2String(typed)
	case int:
		return platformencoding.Interface2String(typed)
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}
