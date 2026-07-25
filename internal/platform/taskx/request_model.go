package taskx

import (
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
)

func CoverTaskActionToModelName(platform constant.TaskPlatform, action string) string {
	return strings.ToLower(string(platform)) + "_" + strings.ToLower(action)
}
