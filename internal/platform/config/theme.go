package config

import (
	"strings"
	"sync/atomic"
)

var themeValue atomic.Value // stores string; safe for concurrent read/write

func init() {
	themeValue.Store("default")
}

func GetTheme() string {
	return themeValue.Load().(string)
}

// SetTheme updates the frontend theme atomically.
// Only "default" is accepted; other values are silently ignored.
func SetTheme(t string) {
	if t == "default" {
		themeValue.Store(t)
	}
}

// ThemeAwarePath rewrites legacy /console/* paths to the default-theme
// equivalents. The function only touches known prefixes so it is safe to call
// with arbitrary suffixes and query strings.
func ThemeAwarePath(suffix string) string {
	switch {
	case strings.HasPrefix(suffix, "/console/topup"):
		return strings.Replace(suffix, "/console/topup", "/wallet", 1)
	case strings.HasPrefix(suffix, "/console/log"):
		return strings.Replace(suffix, "/console/log", "/usage-logs", 1)
	case strings.HasPrefix(suffix, "/console/personal"):
		return strings.Replace(suffix, "/console/personal", "/profile", 1)
	}
	return suffix
}
