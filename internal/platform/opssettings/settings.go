package opssettings

import (
	"strings"
	"sync"
)

var (
	mu sync.RWMutex

	demoSiteEnabled    bool
	selfUseModeEnabled bool

	automaticDisableKeywords = []string{
		"Your credit balance is too low",
		"This organization has been disabled.",
		"You exceeded your current quota",
		"Permission denied",
		"The security token included in the request is invalid",
		"Operation not allowed",
		"Your account is not authorized",
	}
)

// IsDemoSiteEnabled reports whether demo-site mode is enabled.
func IsDemoSiteEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return demoSiteEnabled
}

// SetDemoSiteEnabled updates the demo-site mode flag.
func SetDemoSiteEnabled(enabled bool) {
	mu.Lock()
	defer mu.Unlock()
	demoSiteEnabled = enabled
}

// IsSelfUseModeEnabled reports whether self-use mode is enabled.
func IsSelfUseModeEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return selfUseModeEnabled
}

// SetSelfUseModeEnabled updates the self-use mode flag.
func SetSelfUseModeEnabled(enabled bool) {
	mu.Lock()
	defer mu.Unlock()
	selfUseModeEnabled = enabled
}

// GetAutomaticDisableKeywords returns a copy of the current disable-keyword list.
func GetAutomaticDisableKeywords() []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), automaticDisableKeywords...)
}

// AutomaticDisableKeywordsToString serializes disable keywords into newline-separated text.
func AutomaticDisableKeywordsToString() string {
	mu.RLock()
	defer mu.RUnlock()
	return strings.Join(automaticDisableKeywords, "\n")
}

// SetAutomaticDisableKeywordsFromString replaces disable keywords from newline-separated text.
func SetAutomaticDisableKeywordsFromString(s string) {
	keywords := make([]string, 0)
	for _, keyword := range strings.Split(s, "\n") {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	automaticDisableKeywords = keywords
}
