package store

import "github.com/sh2001sh/CodeGo-Api/setting/config"

type FetchSetting struct {
	EnableSSRFProtection   bool     `json:"enable_ssrf_protection"`
	AllowPrivateIp         bool     `json:"allow_private_ip"`
	DomainFilterMode       bool     `json:"domain_filter_mode"`
	IpFilterMode           bool     `json:"ip_filter_mode"`
	DomainList             []string `json:"domain_list"`
	IpList                 []string `json:"ip_list"`
	AllowedPorts           []string `json:"allowed_ports"`
	ApplyIPFilterForDomain bool     `json:"apply_ip_filter_for_domain"`
}

var defaultFetchSetting = FetchSetting{
	EnableSSRFProtection:   true,
	AllowPrivateIp:         false,
	DomainFilterMode:       false,
	IpFilterMode:           false,
	DomainList:             []string{},
	IpList:                 []string{},
	AllowedPorts:           []string{"80", "443", "8080", "8443"},
	ApplyIPFilterForDomain: true,
}

func init() {
	config.GlobalConfig.Register("fetch_setting", &defaultFetchSetting)
}

func GetFetchSetting() *FetchSetting {
	return &defaultFetchSetting
}
