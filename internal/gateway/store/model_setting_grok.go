package store

import "github.com/sh2001sh/CodeGo-Api/setting/config"

type GrokSettings struct {
	ViolationDeductionEnabled bool    `json:"violation_deduction_enabled"`
	ViolationDeductionAmount  float64 `json:"violation_deduction_amount"`
}

var defaultGrokSettings = GrokSettings{
	ViolationDeductionEnabled: true,
	ViolationDeductionAmount:  0.05,
}

var grokSettings = defaultGrokSettings

func init() {
	config.GlobalConfig.Register("grok", &grokSettings)
}

func GetGrokSettings() *GrokSettings {
	return &grokSettings
}
