package store

import "github.com/sh2001sh/CodeGo-Api/setting/config"

// TokenSetting controls user token limits under the identity module.
type TokenSetting struct {
	MaxUserTokens int `json:"max_user_tokens"`
}

var tokenSetting = TokenSetting{
	MaxUserTokens: 1000,
}

func init() {
	config.GlobalConfig.Register("token_setting", &tokenSetting)
}

func GetTokenSetting() *TokenSetting {
	return &tokenSetting
}

func GetMaxUserTokens() int {
	return GetTokenSetting().MaxUserTokens
}
