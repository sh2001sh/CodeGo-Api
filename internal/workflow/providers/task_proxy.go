package providers

import (
	"fmt"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
)

func BuildTaskProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/videos/%s/content", platformconfig.ServerAddress, taskID)
}
