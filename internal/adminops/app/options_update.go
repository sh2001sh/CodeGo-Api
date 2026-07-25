package app

import (
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	requestsettings "github.com/sh2001sh/CodeGo-Api/internal/platform/requestsettings"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	"strings"
	// UpdateOption validates and persists a runtime option update.
)

func UpdateOption(input AdminOptionUpdateInput) error {
	value := normalizeOptionValue(input.Value)
	if err := validateOptionGuards(input.Key, value); err != nil {
		return err
	}
	if err := validateOptionValue(input.Key, value); err != nil {
		return err
	}
	return platformstore.UpdateOption(input.Key, value)
}

func validateOptionGuards(key string, value string) error {
	if isPaymentComplianceOptionKey(key) {
		return ErrOptionComplianceFieldImmutable
	}
	return nil
}

func validateOptionValue(key string, value string) error {
	switch key {
	case "GitHubOAuthEnabled":
		if value == "true" && platformconfig.GitHubClientId == "" {
			return ErrOptionGitHubOAuthMissingConfig
		}
	case "discord.enabled":
		if value == "true" && platformstore.GetDiscordSettings().ClientId == "" {
			return ErrOptionDiscordOAuthMissingConfig
		}
	case "oidc.enabled":
		if value == "true" && platformstore.GetOIDCSettings().ClientId == "" {
			return ErrOptionOIDCMissingConfig
		}
	case "LinuxDOOAuthEnabled":
		if value == "true" && platformconfig.LinuxDOClientId == "" {
			return ErrOptionLinuxDOMissingConfig
		}
	case "EmailDomainRestrictionEnabled":
		if value == "true" && len(platformconfig.EmailDomainWhitelist) == 0 {
			return ErrOptionEmailDomainMissingConfig
		}
	case "WeChatAuthEnabled":
		if value == "true" && platformconfig.WeChatServerAddress == "" {
			return ErrOptionWeChatMissingConfig
		}
	case "TurnstileCheckEnabled":
		if value == "true" && platformconfig.TurnstileSiteKey == "" {
			return ErrOptionTurnstileMissingConfig
		}
	case "TelegramOAuthEnabled":
		if value == "true" && platformconfig.TelegramBotToken == "" {
			return ErrOptionTelegramMissingConfig
		}
	case "theme.frontend":
		if value != "default" {
			return ErrOptionThemeFrontendInvalid
		}
	case "GroupRatio":
		return gatewaystore.CheckGroupRatio(value)
	case "ImageRatio":
		return gatewaystore.UpdateImageRatioByJSONString(value)
	case "AudioRatio":
		return gatewaystore.UpdateAudioRatioByJSONString(value)
	case "AudioCompletionRatio":
		return gatewaystore.UpdateAudioCompletionRatioByJSONString(value)
	case "CreateCacheRatio":
		return gatewaystore.UpdateCreateCacheRatioByJSONString(value)
	case "ModelRequestRateLimitGroup":
		return requestsettings.CheckModelRequestRateLimitGroup(value)
	case "AutomaticDisableStatusCodes":
		_, err := gatewaystore.ParseHTTPStatusCodeRanges(value)
		return err
	case "AutomaticRetryStatusCodes":
		_, err := gatewaystore.ParseHTTPStatusCodeRanges(value)
		return err
	case "console_setting.api_info":
		return platformstore.ValidateConsoleSettings(value, "ApiInfo")
	case "console_setting.announcements":
		return platformstore.ValidateConsoleSettings(value, "Announcements")
	case "console_setting.faq":
		return platformstore.ValidateConsoleSettings(value, "FAQ")
	case "console_setting.uptime_kuma_groups":
		return platformstore.ValidateConsoleSettings(value, "UptimeKumaGroups")
	}
	return nil
}

// MigrateConsoleSetting migrates legacy console options into the new console_setting.* keys.
func MigrateConsoleSetting() error {
	opts, err := platformstore.ListOptions()
	if err != nil {
		return err
	}

	valueMap := map[string]string{}
	for _, option := range opts {
		valueMap[option.Key] = option.Value
	}

	if value := valueMap["ApiInfo"]; value != "" {
		var items []map[string]any
		if err := platformencoding.UnmarshalString(value, &items); err == nil {
			if len(items) > 50 {
				items = items[:50]
			}
			if bytes, marshalErr := platformencoding.Marshal(items); marshalErr == nil {
				if err := platformstore.UpdateOption("console_setting.api_info", string(bytes)); err != nil {
					return err
				}
			}
		}
		if err := platformstore.UpdateOption("ApiInfo", ""); err != nil {
			return err
		}
	}

	if value := valueMap["Announcements"]; value != "" {
		if err := platformstore.UpdateOption("console_setting.announcements", value); err != nil {
			return err
		}
		if err := platformstore.UpdateOption("Announcements", ""); err != nil {
			return err
		}
	}

	if value := valueMap["FAQ"]; value != "" {
		var items []map[string]any
		if err := platformencoding.UnmarshalString(value, &items); err == nil {
			out := make([]map[string]any, 0, len(items))
			for _, item := range items {
				question, _ := item["question"].(string)
				if question == "" {
					question, _ = item["title"].(string)
				}
				answer, _ := item["answer"].(string)
				if answer == "" {
					answer, _ = item["content"].(string)
				}
				if question != "" && answer != "" {
					out = append(out, map[string]any{"question": question, "answer": answer})
				}
			}
			if len(out) > 50 {
				out = out[:50]
			}
			if bytes, marshalErr := platformencoding.Marshal(out); marshalErr == nil {
				if err := platformstore.UpdateOption("console_setting.faq", string(bytes)); err != nil {
					return err
				}
			}
		}
		if err := platformstore.UpdateOption("FAQ", ""); err != nil {
			return err
		}
	}

	url := strings.TrimSpace(valueMap["UptimeKumaUrl"])
	slug := strings.TrimSpace(valueMap["UptimeKumaSlug"])
	if url != "" && slug != "" {
		groups := []map[string]any{
			{
				"id":           1,
				"categoryName": "old",
				"url":          url,
				"slug":         slug,
				"description":  "",
			},
		}
		if bytes, marshalErr := platformencoding.Marshal(groups); marshalErr == nil {
			if err := platformstore.UpdateOption("console_setting.uptime_kuma_groups", string(bytes)); err != nil {
				return err
			}
		}
	}
	if url != "" {
		if err := platformstore.UpdateOption("UptimeKumaUrl", ""); err != nil {
			return err
		}
	}
	if slug != "" {
		if err := platformstore.UpdateOption("UptimeKumaSlug", ""); err != nil {
			return err
		}
	}

	oldKeys := []string{"ApiInfo", "Announcements", "FAQ", "UptimeKumaUrl", "UptimeKumaSlug"}
	if err := platformdb.DB.Where("key IN ?", oldKeys).Delete(&platformschema.Option{}).Error; err != nil {
		return err
	}
	platformstore.InitOptionMap()
	platformobservability.SysLog("console setting migrated")
	return nil
}
