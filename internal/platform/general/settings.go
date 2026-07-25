package general

import settingconfig "github.com/sh2001sh/CodeGo-Api/setting/config"

const (
	QuotaDisplayTypeUSD    = "USD"
	QuotaDisplayTypeCNY    = "CNY"
	QuotaDisplayTypeTokens = "TOKENS"
	QuotaDisplayTypeCustom = "CUSTOM"
)

type Setting struct {
	DocsLink                   string  `json:"docs_link"`
	PingIntervalEnabled        bool    `json:"ping_interval_enabled"`
	PingIntervalSeconds        int     `json:"ping_interval_seconds"`
	QuotaDisplayType           string  `json:"quota_display_type"`
	CustomCurrencySymbol       string  `json:"custom_currency_symbol"`
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
}

var currentSetting = Setting{
	DocsLink:                   "",
	PingIntervalEnabled:        true,
	PingIntervalSeconds:        60,
	QuotaDisplayType:           QuotaDisplayTypeUSD,
	CustomCurrencySymbol:       "¤",
	CustomCurrencyExchangeRate: 1.0,
}

func init() {
	settingconfig.GlobalConfig.Register("general_setting", &currentSetting)
}

func GetSetting() *Setting {
	return &currentSetting
}

func IsCurrencyDisplay() bool {
	return currentSetting.QuotaDisplayType != QuotaDisplayTypeTokens
}

func IsCNYDisplay() bool {
	return currentSetting.QuotaDisplayType == QuotaDisplayTypeCNY
}

func GetQuotaDisplayType() string {
	return currentSetting.QuotaDisplayType
}

func GetCurrencySymbol() string {
	switch currentSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return "$"
	case QuotaDisplayTypeCNY:
		return "¥"
	case QuotaDisplayTypeCustom:
		if currentSetting.CustomCurrencySymbol != "" {
			return currentSetting.CustomCurrencySymbol
		}
		return "¤"
	default:
		return ""
	}
}

func GetUsdToCurrencyRate(usdToCny float64) float64 {
	switch currentSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return 1
	case QuotaDisplayTypeCNY:
		return usdToCny
	case QuotaDisplayTypeCustom:
		if currentSetting.CustomCurrencyExchangeRate > 0 {
			return currentSetting.CustomCurrencyExchangeRate
		}
		return 1
	default:
		return 1
	}
}
