package store

import (
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	clientlinks "github.com/sh2001sh/CodeGo-Api/internal/platform/clientlinks"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	requestsettings "github.com/sh2001sh/CodeGo-Api/internal/platform/requestsettings"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/setting/config"
	"strconv"
	"strings"
)

func buildDefaultOptionMap() map[string]string {
	optionMap := map[string]string{
		"FileUploadPermission":                 strconv.Itoa(platformconfig.FileUploadPermission),
		"FileDownloadPermission":               strconv.Itoa(platformconfig.FileDownloadPermission),
		"ImageUploadPermission":                strconv.Itoa(platformconfig.ImageUploadPermission),
		"ImageDownloadPermission":              strconv.Itoa(platformconfig.ImageDownloadPermission),
		"PasswordLoginEnabled":                 strconv.FormatBool(platformconfig.PasswordLoginEnabled),
		"PasswordRegisterEnabled":              strconv.FormatBool(platformconfig.PasswordRegisterEnabled),
		"EmailVerificationEnabled":             strconv.FormatBool(platformconfig.EmailVerificationEnabled),
		"GitHubOAuthEnabled":                   strconv.FormatBool(platformconfig.GitHubOAuthEnabled),
		"LinuxDOOAuthEnabled":                  strconv.FormatBool(platformconfig.LinuxDOOAuthEnabled),
		"TelegramOAuthEnabled":                 strconv.FormatBool(platformconfig.TelegramOAuthEnabled),
		"WeChatAuthEnabled":                    strconv.FormatBool(platformconfig.WeChatAuthEnabled),
		"TurnstileCheckEnabled":                strconv.FormatBool(platformconfig.TurnstileCheckEnabled),
		"RegisterEnabled":                      strconv.FormatBool(platformconfig.RegisterEnabled),
		"AutomaticDisableChannelEnabled":       strconv.FormatBool(platformconfig.AutomaticDisableChannelEnabled),
		"AutomaticEnableChannelEnabled":        strconv.FormatBool(platformconfig.AutomaticEnableChannelEnabled),
		"LogConsumeEnabled":                    strconv.FormatBool(platformconfig.LogConsumeEnabled),
		"DisplayInCurrencyEnabled":             strconv.FormatBool(platformconfig.DisplayInCurrencyEnabled),
		"DisplayTokenStatEnabled":              strconv.FormatBool(platformconfig.DisplayTokenStatEnabled),
		"DrawingEnabled":                       strconv.FormatBool(platformconfig.DrawingEnabled),
		"TaskEnabled":                          strconv.FormatBool(platformconfig.TaskEnabled),
		"DataExportEnabled":                    strconv.FormatBool(platformconfig.DataExportEnabled),
		"ChannelDisableThreshold":              strconv.FormatFloat(platformconfig.ChannelDisableThreshold, 'f', -1, 64),
		"EmailDomainRestrictionEnabled":        strconv.FormatBool(platformconfig.EmailDomainRestrictionEnabled),
		"EmailAliasRestrictionEnabled":         strconv.FormatBool(platformconfig.EmailAliasRestrictionEnabled),
		"EmailDomainWhitelist":                 strings.Join(platformconfig.EmailDomainWhitelist, ","),
		"SMTPServer":                           "",
		"SMTPFrom":                             "",
		"SMTPPort":                             strconv.Itoa(platformconfig.SMTPPort),
		"SMTPAccount":                          "",
		"SMTPToken":                            "",
		"SMTPSSLEnabled":                       strconv.FormatBool(platformconfig.SMTPSSLEnabled),
		"SMTPForceAuthLogin":                   strconv.FormatBool(platformconfig.SMTPForceAuthLogin),
		"Notice":                               "",
		"About":                                "",
		"HomePageContent":                      "",
		"HomePagePackagesContent":              "",
		"Footer":                               platformconfig.Footer,
		"SystemName":                           platformconfig.SystemName,
		"Logo":                                 platformconfig.Logo,
		"ServerAddress":                        "",
		"WorkerUrl":                            platformconfig.WorkerUrl,
		"WorkerValidKey":                       platformconfig.WorkerValidKey,
		"WorkerAllowHttpImageRequestEnabled":   strconv.FormatBool(platformconfig.WorkerAllowHttpImageRequestEnabled),
		"CustomCallbackAddress":                "",
		"Price":                                strconv.FormatFloat(commercestore.Price, 'f', -1, 64),
		"USDExchangeRate":                      strconv.FormatFloat(commercestore.USDExchangeRate, 'f', -1, 64),
		"MinTopUp":                             strconv.Itoa(commercestore.MinTopUp),
		"StripeMinTopUp":                       strconv.Itoa(commercestore.StripeMinTopUp),
		"StripeApiSecret":                      commercestore.StripeApiSecret,
		"StripeWebhookSecret":                  commercestore.StripeWebhookSecret,
		"StripePriceId":                        commercestore.StripePriceId,
		"StripeUnitPrice":                      strconv.FormatFloat(commercestore.StripeUnitPrice, 'f', -1, 64),
		"StripePromotionCodesEnabled":          strconv.FormatBool(commercestore.StripePromotionCodesEnabled),
		"CreemApiKey":                          commercestore.CreemApiKey,
		"CreemProducts":                        commercestore.CreemProducts,
		"CreemTestMode":                        strconv.FormatBool(commercestore.CreemTestMode),
		"CreemWebhookSecret":                   commercestore.CreemWebhookSecret,
		"TopupGroupRatio":                      commercedomain.TopupGroupRatioJSON(),
		"Chats":                                clientlinks.Chats2JsonString(),
		"AutoGroups":                           gatewaygroups.AutoGroups2JsonString(),
		"DefaultUseAutoGroup":                  strconv.FormatBool(gatewaygroups.DefaultUseAutoGroup),
		"GitHubClientId":                       "",
		"GitHubClientSecret":                   "",
		"TelegramBotToken":                     "",
		"TelegramBotName":                      "",
		"WeChatServerAddress":                  "",
		"WeChatServerToken":                    "",
		"WeChatAccountQRCodeImageURL":          "",
		"TurnstileSiteKey":                     "",
		"TurnstileSecretKey":                   "",
		"QuotaForNewUser":                      strconv.Itoa(platformconfig.QuotaForNewUser),
		"QuotaRemindThreshold":                 strconv.Itoa(platformconfig.QuotaRemindThreshold),
		"PreConsumedQuota":                     strconv.Itoa(platformconfig.PreConsumedQuota),
		"ModelRequestRateLimitCount":           strconv.Itoa(requestsettings.ModelRequestRateLimitCount),
		"ModelRequestRateLimitDurationMinutes": strconv.Itoa(requestsettings.ModelRequestRateLimitDurationMinutes),
		"ModelRequestRateLimitSuccessCount":    strconv.Itoa(requestsettings.ModelRequestRateLimitSuccessCount),
		"ModelRequestRateLimitGroup":           requestsettings.ModelRequestRateLimitGroup2JSONString(),
		"ModelRatio":                           gatewaystore.ModelRatio2JSONString(),
		"ModelPrice":                           gatewaystore.ModelPrice2JSONString(),
		"CacheRatio":                           gatewaystore.CacheRatio2JSONString(),
		"CreateCacheRatio":                     gatewaystore.CreateCacheRatio2JSONString(),
		"GroupRatio":                           gatewaystore.GroupRatio2JSONString(),
		"GroupGroupRatio":                      gatewaystore.GroupGroupRatio2JSONString(),
		"UserUsableGroups":                     gatewaygroups.UserUsableGroups2JSONString(),
		"CompletionRatio":                      gatewaystore.CompletionRatio2JSONString(),
		"ImageRatio":                           gatewaystore.ImageRatio2JSONString(),
		"AudioRatio":                           gatewaystore.AudioRatio2JSONString(),
		"AudioCompletionRatio":                 gatewaystore.AudioCompletionRatio2JSONString(),
		"TopUpLink":                            platformconfig.TopUpLink,
		"QuotaPerUnit":                         strconv.FormatFloat(platformruntime.QuotaPerUnit, 'f', -1, 64),
		"RetryTimes":                           strconv.Itoa(platformconfig.RetryTimes),
		"DataExportInterval":                   strconv.Itoa(platformconfig.DataExportInterval),
		"DataExportDefaultTime":                platformconfig.DataExportDefaultTime,
		"DefaultCollapseSidebar":               strconv.FormatBool(platformconfig.DefaultCollapseSidebar),
		"CheckSensitiveEnabled":                strconv.FormatBool(requestsettings.CheckSensitiveEnabled),
		"DemoSiteEnabled":                      strconv.FormatBool(platformops.IsDemoSiteEnabled()),
		"SelfUseModeEnabled":                   strconv.FormatBool(platformops.IsSelfUseModeEnabled()),
		"ModelRequestRateLimitEnabled":         strconv.FormatBool(requestsettings.ModelRequestRateLimitEnabled),
		"CheckSensitiveOnPromptEnabled":        strconv.FormatBool(requestsettings.CheckSensitiveOnPromptEnabled),
		"StopOnSensitiveEnabled":               strconv.FormatBool(requestsettings.StopOnSensitiveEnabled),
		"SensitiveWords":                       requestsettings.SensitiveWordsToString(),
		"StreamCacheQueueLength":               strconv.Itoa(requestsettings.StreamCacheQueueLength),
		"AutomaticDisableKeywords":             platformops.AutomaticDisableKeywordsToString(),
		"AutomaticDisableStatusCodes":          gatewaystore.AutomaticDisableStatusCodesToString(),
		"AutomaticRetryStatusCodes":            gatewaystore.AutomaticRetryStatusCodesToString(),
		"ExposeRatioEnabled":                   strconv.FormatBool(gatewaystore.IsExposeRatioEnabled()),
	}

	for key, value := range config.GlobalConfig.ExportAllConfigs() {
		optionMap[key] = value
	}

	return optionMap
}
