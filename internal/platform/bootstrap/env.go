package bootstrap

import (
	"os"
	"strings"
)

func getenvTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
