package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	clientlinks "github.com/sh2001sh/CodeGo-Api/internal/platform/clientlinks"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformgeneral "github.com/sh2001sh/CodeGo-Api/internal/platform/general"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

// TestStatusSnapshot exposes a lightweight admin-only runtime health snapshot.
type TestStatusSnapshot struct {
	Success   bool `json:"success"`
	HTTPStats any  `json:"http_stats"`
}

// CustomOAuthProviderInfo exposes the public subset of a custom OAuth provider.
type CustomOAuthProviderInfo struct {
	Id                    int    `json:"id"`
	Name                  string `json:"name"`
	Slug                  string `json:"slug"`
	Icon                  string `json:"icon"`
	ClientId              string `json:"client_id"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	Scopes                string `json:"scopes"`
}

// GetPublicStatus returns the public runtime status payload consumed by the web shell.
func GetPublicStatus() map[string]any {
	cs := platformstore.GetConsoleSetting()
	passkeySetting := platformstore.GetPasskeySettings()
	legalSetting := platformstore.GetLegalSettings()

	platformconfig.OptionMapRWMutex.RLock()
	defer platformconfig.OptionMapRWMutex.RUnlock()

	data := map[string]any{
		"version":                       platformconfig.Version,
		"start_time":                    platformconfig.StartTime,
		"email_verification":            platformconfig.EmailVerificationEnabled,
		"github_oauth":                  platformconfig.GitHubOAuthEnabled,
		"github_client_id":              platformconfig.GitHubClientId,
		"discord_oauth":                 platformstore.GetDiscordSettings().Enabled,
		"discord_client_id":             platformstore.GetDiscordSettings().ClientId,
		"linuxdo_oauth":                 platformconfig.LinuxDOOAuthEnabled,
		"linuxdo_client_id":             platformconfig.LinuxDOClientId,
		"linuxdo_minimum_trust_level":   platformconfig.LinuxDOMinimumTrustLevel,
		"telegram_oauth":                platformconfig.TelegramOAuthEnabled,
		"telegram_bot_name":             platformconfig.TelegramBotName,
		"theme":                         platformstore.GetThemeSettings().Frontend,
		"system_name":                   platformconfig.SystemName,
		"logo":                          platformconfig.Logo,
		"footer_html":                   platformconfig.Footer,
		"wechat_qrcode":                 platformconfig.WeChatAccountQRCodeImageURL,
		"wechat_login":                  platformconfig.WeChatAuthEnabled,
		"server_address":                platformconfig.ServerAddress,
		"turnstile_check":               platformconfig.TurnstileCheckEnabled,
		"turnstile_site_key":            platformconfig.TurnstileSiteKey,
		"docs_link":                     platformgeneral.GetSetting().DocsLink,
		"quota_per_unit":                platformruntime.QuotaPerUnit,
		"display_in_currency":           platformgeneral.IsCurrencyDisplay(),
		"quota_display_type":            platformgeneral.GetQuotaDisplayType(),
		"custom_currency_symbol":        platformgeneral.GetSetting().CustomCurrencySymbol,
		"custom_currency_exchange_rate": platformgeneral.GetSetting().CustomCurrencyExchangeRate,
		"enable_batch_update":           platformconfig.BatchUpdateEnabled,
		"enable_drawing":                platformconfig.DrawingEnabled,
		"enable_task":                   platformconfig.TaskEnabled,
		"enable_data_export":            platformconfig.DataExportEnabled,
		"data_export_default_time":      platformconfig.DataExportDefaultTime,
		"default_collapse_sidebar":      platformconfig.DefaultCollapseSidebar,
		"chats":                         clientlinks.Chats,
		"demo_site_enabled":             platformops.IsDemoSiteEnabled(),
		"self_use_mode_enabled":         platformops.IsSelfUseModeEnabled(),
		"default_use_auto_group":        gatewaygroups.DefaultUseAutoGroup,
		"usd_exchange_rate":             commercestore.USDExchangeRate,
		"price":                         commercestore.Price,
		"stripe_unit_price":             commercestore.StripeUnitPrice,
		"api_info_enabled":              cs.ApiInfoEnabled,
		"uptime_kuma_enabled":           cs.UptimeKumaEnabled,
		"announcements_enabled":         cs.AnnouncementsEnabled,
		"faq_enabled":                   cs.FAQEnabled,
		"HeaderNavModules":              platformconfig.OptionMap["HeaderNavModules"],
		"SidebarModulesAdmin":           platformconfig.OptionMap["SidebarModulesAdmin"],
		"oidc_enabled":                  platformstore.GetOIDCSettings().Enabled,
		"oidc_client_id":                platformstore.GetOIDCSettings().ClientId,
		"oidc_authorization_endpoint":   platformstore.GetOIDCSettings().AuthorizationEndpoint,
		"passkey_login":                 passkeySetting.Enabled,
		"passkey_display_name":          passkeySetting.RPDisplayName,
		"passkey_rp_id":                 passkeySetting.RPID,
		"passkey_origins":               passkeySetting.Origins,
		"passkey_allow_insecure":        passkeySetting.AllowInsecureOrigin,
		"passkey_user_verification":     passkeySetting.UserVerification,
		"passkey_attachment":            passkeySetting.AttachmentPreference,
		"setup":                         constant.Setup,
		"user_agreement_enabled":        legalSetting.UserAgreement != "",
		"privacy_policy_enabled":        legalSetting.PrivacyPolicy != "",
	}

	if cs.ApiInfoEnabled {
		data["api_info"] = platformstore.GetAPIInfo()
	}
	if cs.AnnouncementsEnabled {
		data["announcements"] = platformstore.GetAnnouncements()
	}
	if cs.FAQEnabled {
		data["faq"] = platformstore.GetFAQ()
	}

	customProviders := oauth.GetEnabledCustomProviders()
	if len(customProviders) > 0 {
		providersInfo := make([]CustomOAuthProviderInfo, 0, len(customProviders))
		for _, provider := range customProviders {
			config := provider.GetConfig()
			providersInfo = append(providersInfo, CustomOAuthProviderInfo{
				Id:                    config.Id,
				Name:                  config.Name,
				Slug:                  config.Slug,
				Icon:                  config.Icon,
				ClientId:              config.ClientId,
				AuthorizationEndpoint: config.AuthorizationEndpoint,
				Scopes:                config.Scopes,
			})
		}
		data["custom_oauth_providers"] = providersInfo
	}

	return data
}
