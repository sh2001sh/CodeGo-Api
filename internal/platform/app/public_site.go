package app

import (
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

// GetNotice returns the current public notice.
func GetNotice() string {
	return getOptionString("Notice")
}

// GetAbout returns the public about content.
func GetAbout() string {
	return getOptionString("About")
}

// GetUserAgreement returns the current user agreement.
func GetUserAgreement() string {
	return platformstore.GetLegalSettings().UserAgreement
}

// GetPrivacyPolicy returns the current privacy policy.
func GetPrivacyPolicy() string {
	return platformstore.GetLegalSettings().PrivacyPolicy
}

// GetHomePageContent returns the public homepage content block.
func GetHomePageContent() string {
	return getOptionString("HomePageContent")
}

// GetHomePagePackagesContent returns the public homepage packages content block.
func GetHomePagePackagesContent() string {
	return getOptionString("HomePagePackagesContent")
}

func getOptionString(key string) string {
	platformconfig.OptionMapRWMutex.RLock()
	defer platformconfig.OptionMapRWMutex.RUnlock()
	return platformconfig.OptionMap[key]
}
