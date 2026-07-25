package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// GetEnvOrDefaultInt returns the parsed integer value for key or fallback when unset/invalid.
func GetEnvOrDefaultInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("failed to parse %s=%q, using default value: %d", key, raw, fallback)
		return fallback
	}
	return value
}

// GetEnvOrDefaultString returns the environment value for key or fallback when unset.
func GetEnvOrDefaultString(key string, fallback string) string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

// GetEnvOrDefaultBool returns the parsed boolean value for key or fallback when unset/invalid.
func GetEnvOrDefaultBool(key string, fallback bool) bool {
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
