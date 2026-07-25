package config

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var (
	sessionCookieSecure      bool
	sessionCookieTrustedURLs []string
)

func InitSessionCookieConfig() {
	sessionCookieSecure = getEnvBool("SESSION_COOKIE_SECURE", false)

	rawTrusted := strings.TrimSpace(os.Getenv("SESSION_COOKIE_TRUSTED_URL"))
	sessionCookieTrustedURLs = normalizeSessionCookieTrustedURLs(rawTrusted)

	if sessionCookieSecure {
		return
	}
	if len(sessionCookieTrustedURLs) == 0 {
		log.Println("SESSION_COOKIE_SECURE is disabled; session cookies will also be sent over HTTP")
		return
	}
	log.Println("SESSION_COOKIE_SECURE is disabled even though trusted HTTPS URLs are configured; review your deployment proxy/TLS setup")
}

func SessionCookieSecure() bool {
	return sessionCookieSecure
}

func SessionCookieTrustedURLs() []string {
	if len(sessionCookieTrustedURLs) == 0 {
		return nil
	}
	copied := make([]string, len(sessionCookieTrustedURLs))
	copy(copied, sessionCookieTrustedURLs)
	return copied
}

func normalizeSessionCookieTrustedURLs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if !strings.Contains(candidate, "://") {
			candidate = "https://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			continue
		}
		normalized := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
		if parsed.Path != "" && parsed.Path != "/" {
			normalized += strings.TrimRight(parsed.Path, "/")
		}
		result = append(result, normalized)
	}
	return result
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("failed to parse %s=%q, using default value: %t", key, raw, fallback)
		return fallback
	}
	return value
}
