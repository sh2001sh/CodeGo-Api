package store

import "github.com/sh2001sh/CodeGo-Api/setting/config"

type ConsoleSetting struct {
	ApiInfo              string `json:"api_info"`
	UptimeKumaGroups     string `json:"uptime_kuma_groups"`
	Announcements        string `json:"announcements"`
	FAQ                  string `json:"faq"`
	ApiInfoEnabled       bool   `json:"api_info_enabled"`
	UptimeKumaEnabled    bool   `json:"uptime_kuma_enabled"`
	AnnouncementsEnabled bool   `json:"announcements_enabled"`
	FAQEnabled           bool   `json:"faq_enabled"`
}

var defaultConsoleSetting = ConsoleSetting{
	ApiInfo:              "",
	UptimeKumaGroups:     "",
	Announcements:        "",
	FAQ:                  "",
	ApiInfoEnabled:       true,
	UptimeKumaEnabled:    true,
	AnnouncementsEnabled: true,
	FAQEnabled:           true,
}

var consoleSetting = defaultConsoleSetting

func init() {
	config.GlobalConfig.Register("console_setting", &consoleSetting)
}

func GetConsoleSetting() *ConsoleSetting {
	return &consoleSetting
}
