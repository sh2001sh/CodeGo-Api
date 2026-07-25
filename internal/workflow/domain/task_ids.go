package domain

import (
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

// GeneratePublicTaskID creates the public task_xxxx identifier exposed outside the system.
func GeneratePublicTaskID() string {
	key, _ := platformruntime.GenerateRandomCharsKey(32)
	return "task_" + key
}
