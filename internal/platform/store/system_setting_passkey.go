package store

import (
	"net/url"
	"strings"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/setting/config"
)

type PasskeySettings struct {
	Enabled              bool   `json:"enabled"`
	RPDisplayName        string `json:"rp_display_name"`
	RPID                 string `json:"rp_id"`
	Origins              string `json:"origins"`
	AllowInsecureOrigin  bool   `json:"allow_insecure_origin"`
	UserVerification     string `json:"user_verification"`
	AttachmentPreference string `json:"attachment_preference"`
}

var defaultPasskeySettings = PasskeySettings{
	Enabled:              false,
	RPDisplayName:        platformconfig.SystemName,
	RPID:                 "",
	Origins:              "",
	AllowInsecureOrigin:  false,
	UserVerification:     "preferred",
	AttachmentPreference: "",
}

func init() {
	config.GlobalConfig.Register("passkey", &defaultPasskeySettings)
}

func GetPasskeySettings() *PasskeySettings {
	if defaultPasskeySettings.RPID == "" && platformconfig.ServerAddress != "" {
		serverAddr := strings.TrimSpace(platformconfig.ServerAddress)
		if parsed, err := url.Parse(serverAddr); err == nil && parsed.Host != "" {
			defaultPasskeySettings.RPID = parsed.Host
		} else {
			defaultPasskeySettings.RPID = serverAddr
		}
	}
	if defaultPasskeySettings.Origins == "" || defaultPasskeySettings.Origins == "[]" {
		defaultPasskeySettings.Origins = platformconfig.ServerAddress
	}
	return &defaultPasskeySettings
}
