package store

import (
	toolpricing "github.com/sh2001sh/CodeGo-Api/internal/billing/toolpricing"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	clientlinks "github.com/sh2001sh/CodeGo-Api/internal/platform/clientlinks"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	requestsettings "github.com/sh2001sh/CodeGo-Api/internal/platform/requestsettings"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"strconv"
	"strings"

	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/setting/config"
)

func applyOptionValue(key string, value string) (err error) {
	platformconfig.OptionMapRWMutex.Lock()
	defer platformconfig.OptionMapRWMutex.Unlock()
	platformconfig.OptionMap[key] = value

	if handleConfigUpdate(key, value) {
		return nil
	}

	if strings.HasSuffix(key, "Permission") {
		intValue, _ := strconv.Atoi(value)
		switch key {
		case "FileUploadPermission":
			platformconfig.FileUploadPermission = intValue
		case "FileDownloadPermission":
			platformconfig.FileDownloadPermission = intValue
		case "ImageUploadPermission":
			platformconfig.ImageUploadPermission = intValue
		case "ImageDownloadPermission":
			platformconfig.ImageDownloadPermission = intValue
		}
	}

	if strings.HasSuffix(key, "Enabled") || key == "DefaultCollapseSidebar" || key == "DefaultUseAutoGroup" || key == "SMTPForceAuthLogin" {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			platformconfig.PasswordRegisterEnabled = boolValue
		case "PasswordLoginEnabled":
			platformconfig.PasswordLoginEnabled = boolValue
		case "EmailVerificationEnabled":
			platformconfig.EmailVerificationEnabled = boolValue
		case "GitHubOAuthEnabled":
			platformconfig.GitHubOAuthEnabled = boolValue
		case "LinuxDOOAuthEnabled":
			platformconfig.LinuxDOOAuthEnabled = boolValue
		case "WeChatAuthEnabled":
			platformconfig.WeChatAuthEnabled = boolValue
		case "TelegramOAuthEnabled":
			platformconfig.TelegramOAuthEnabled = boolValue
		case "TurnstileCheckEnabled":
			platformconfig.TurnstileCheckEnabled = boolValue
		case "RegisterEnabled":
			platformconfig.RegisterEnabled = boolValue
		case "EmailDomainRestrictionEnabled":
			platformconfig.EmailDomainRestrictionEnabled = boolValue
		case "EmailAliasRestrictionEnabled":
			platformconfig.EmailAliasRestrictionEnabled = boolValue
		case "AutomaticDisableChannelEnabled":
			platformconfig.AutomaticDisableChannelEnabled = boolValue
		case "AutomaticEnableChannelEnabled":
			platformconfig.AutomaticEnableChannelEnabled = boolValue
		case "LogConsumeEnabled":
			platformconfig.LogConsumeEnabled = boolValue
		case "DisplayInCurrencyEnabled":
			quotaDisplayType := "USD"
			if !boolValue {
				quotaDisplayType = "TOKENS"
			}
			if cfg := config.GlobalConfig.Get("general_setting"); cfg != nil {
				_ = config.UpdateConfigFromMap(cfg, map[string]string{"quota_display_type": quotaDisplayType})
			}
		case "DisplayTokenStatEnabled":
			platformconfig.DisplayTokenStatEnabled = boolValue
		case "DrawingEnabled":
			platformconfig.DrawingEnabled = boolValue
		case "TaskEnabled":
			platformconfig.TaskEnabled = boolValue
		case "DataExportEnabled":
			platformconfig.DataExportEnabled = boolValue
		case "DefaultCollapseSidebar":
			platformconfig.DefaultCollapseSidebar = boolValue
		case "CheckSensitiveEnabled":
			requestsettings.CheckSensitiveEnabled = boolValue
		case "DemoSiteEnabled":
			platformops.SetDemoSiteEnabled(boolValue)
		case "SelfUseModeEnabled":
			platformops.SetSelfUseModeEnabled(boolValue)
		case "CheckSensitiveOnPromptEnabled":
			requestsettings.CheckSensitiveOnPromptEnabled = boolValue
		case "ModelRequestRateLimitEnabled":
			requestsettings.ModelRequestRateLimitEnabled = boolValue
		case "StopOnSensitiveEnabled":
			requestsettings.StopOnSensitiveEnabled = boolValue
		case "SMTPSSLEnabled":
			platformconfig.SMTPSSLEnabled = boolValue
		case "SMTPForceAuthLogin":
			platformconfig.SMTPForceAuthLogin = boolValue
		case "WorkerAllowHttpImageRequestEnabled":
			platformconfig.WorkerAllowHttpImageRequestEnabled = boolValue
		case "DefaultUseAutoGroup":
			gatewaygroups.DefaultUseAutoGroup = boolValue
		case "ExposeRatioEnabled":
			gatewaystore.SetExposeRatioEnabled(boolValue)
		}
	}

	switch key {
	case "EmailDomainWhitelist":
		platformconfig.EmailDomainWhitelist = strings.Split(value, ",")
	case "SMTPServer":
		platformconfig.SMTPServer = value
	case "SMTPPort":
		platformconfig.SMTPPort, _ = strconv.Atoi(value)
	case "SMTPAccount":
		platformconfig.SMTPAccount = value
	case "SMTPFrom":
		platformconfig.SMTPFrom = value
	case "SMTPToken":
		platformconfig.SMTPToken = value
	case "ServerAddress":
		platformconfig.ServerAddress = value
	case "WorkerUrl":
		platformconfig.WorkerUrl = value
	case "WorkerValidKey":
		platformconfig.WorkerValidKey = value
	case "Chats":
		err = clientlinks.UpdateChatsByJsonString(value)
	case "AutoGroups":
		err = gatewaygroups.UpdateAutoGroupsByJsonString(value)
	case "CustomCallbackAddress":
		commercestore.CustomCallbackAddress = value
	case "Price":
		commercestore.Price, _ = strconv.ParseFloat(value, 64)
	case "USDExchangeRate":
		commercestore.USDExchangeRate, _ = strconv.ParseFloat(value, 64)
	case "MinTopUp":
		commercestore.MinTopUp, _ = strconv.Atoi(value)
	case "StripeApiSecret":
		commercestore.StripeApiSecret = value
	case "StripeWebhookSecret":
		commercestore.StripeWebhookSecret = value
	case "StripePriceId":
		commercestore.StripePriceId = value
	case "StripeUnitPrice":
		commercestore.StripeUnitPrice, _ = strconv.ParseFloat(value, 64)
	case "StripeMinTopUp":
		commercestore.StripeMinTopUp, _ = strconv.Atoi(value)
	case "StripePromotionCodesEnabled":
		commercestore.StripePromotionCodesEnabled = value == "true"
	case "CreemApiKey":
		commercestore.CreemApiKey = value
	case "CreemProducts":
		commercestore.CreemProducts = value
	case "CreemTestMode":
		commercestore.CreemTestMode = value == "true"
	case "CreemWebhookSecret":
		commercestore.CreemWebhookSecret = value

	case "TopupGroupRatio":
		err = commercedomain.UpdateTopupGroupRatio(value)
	case "GitHubClientId":
		platformconfig.GitHubClientId = value
	case "GitHubClientSecret":
		platformconfig.GitHubClientSecret = value
	case "LinuxDOClientId":
		platformconfig.LinuxDOClientId = value
	case "LinuxDOClientSecret":
		platformconfig.LinuxDOClientSecret = value
	case "LinuxDOMinimumTrustLevel":
		platformconfig.LinuxDOMinimumTrustLevel, _ = strconv.Atoi(value)
	case "Footer":
		platformconfig.Footer = value
	case "SystemName":
		platformconfig.SystemName = value
	case "Logo":
		platformconfig.Logo = value
	case "WeChatServerAddress":
		platformconfig.WeChatServerAddress = value
	case "WeChatServerToken":
		platformconfig.WeChatServerToken = value
	case "WeChatAccountQRCodeImageURL":
		platformconfig.WeChatAccountQRCodeImageURL = value
	case "TelegramBotToken":
		platformconfig.TelegramBotToken = value
	case "TelegramBotName":
		platformconfig.TelegramBotName = value
	case "TurnstileSiteKey":
		platformconfig.TurnstileSiteKey = value
	case "TurnstileSecretKey":
		platformconfig.TurnstileSecretKey = value
	case "QuotaForNewUser":
		platformconfig.QuotaForNewUser, _ = strconv.Atoi(value)
	case "QuotaRemindThreshold":
		platformconfig.QuotaRemindThreshold, _ = strconv.Atoi(value)
	case "PreConsumedQuota":
		platformconfig.PreConsumedQuota, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitCount":
		requestsettings.ModelRequestRateLimitCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitDurationMinutes":
		requestsettings.ModelRequestRateLimitDurationMinutes, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitSuccessCount":
		requestsettings.ModelRequestRateLimitSuccessCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitGroup":
		err = requestsettings.UpdateModelRequestRateLimitGroupByJSONString(value)
	case "RetryTimes":
		platformconfig.RetryTimes, _ = strconv.Atoi(value)
	case "DataExportInterval":
		platformconfig.DataExportInterval, _ = strconv.Atoi(value)
	case "DataExportDefaultTime":
		platformconfig.DataExportDefaultTime = value
	case "ModelRatio":
		err = gatewaystore.UpdateModelRatioByJSONString(value)
	case "GroupRatio":
		err = gatewaystore.UpdateGroupRatioByJSONString(value)
	case "GroupGroupRatio":
		err = gatewaystore.UpdateGroupGroupRatioByJSONString(value)
	case "UserUsableGroups":
		err = gatewaygroups.UpdateUserUsableGroupsByJSONString(value)
	case "CompletionRatio":
		err = gatewaystore.UpdateCompletionRatioByJSONString(value)
	case "ModelPrice":
		err = gatewaystore.UpdateModelPriceByJSONString(value)
	case "CacheRatio":
		err = gatewaystore.UpdateCacheRatioByJSONString(value)
	case "CreateCacheRatio":
		err = gatewaystore.UpdateCreateCacheRatioByJSONString(value)
	case "ImageRatio":
		err = gatewaystore.UpdateImageRatioByJSONString(value)
	case "AudioRatio":
		err = gatewaystore.UpdateAudioRatioByJSONString(value)
	case "AudioCompletionRatio":
		err = gatewaystore.UpdateAudioCompletionRatioByJSONString(value)
	case "TopUpLink":
		platformconfig.TopUpLink = value
	case "ChannelDisableThreshold":
		platformconfig.ChannelDisableThreshold, _ = strconv.ParseFloat(value, 64)
	case "QuotaPerUnit":
		platformruntime.QuotaPerUnit, _ = strconv.ParseFloat(value, 64)
	case "SensitiveWords":
		requestsettings.SensitiveWordsFromString(value)
	case "AutomaticDisableKeywords":
		platformops.SetAutomaticDisableKeywordsFromString(value)
	case "AutomaticDisableStatusCodes":
		err = gatewaystore.AutomaticDisableStatusCodesFromString(value)
	case "AutomaticRetryStatusCodes":
		err = gatewaystore.AutomaticRetryStatusCodesFromString(value)
	case "StreamCacheQueueLength":
		requestsettings.StreamCacheQueueLength, _ = strconv.Atoi(value)
	}
	return err
}

func handleConfigUpdate(key, value string) bool {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false
	}

	configName := parts[0]
	configKey := parts[1]
	cfg := config.GlobalConfig.Get(configName)
	if cfg == nil {
		return false
	}

	config.UpdateConfigFromMap(cfg, map[string]string{
		configKey: value,
	})

	switch configName {
	case "performance_setting":
		updatePerformanceSettingAndSync()
	case "tool_price_setting":
		toolpricing.RebuildIndex()
	case "billing_setting":
		gatewaystore.RestoreMissingDefaultBillingRules()
		gatewaystore.InvalidatePricingCache()
		gatewaystore.InvalidateExposedDataCache()
	case "theme":
		UpdateAndSyncTheme()
	}

	return true
}
