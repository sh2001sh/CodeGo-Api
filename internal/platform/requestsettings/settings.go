package requestsettings

import (
	"encoding/json"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"math"
	"strings"
	"sync"
)

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true
var StopOnSensitiveEnabled = true
var StreamCacheQueueLength = 0

var SensitiveWords = []string{
	"test_sensitive",
}

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000

var (
	modelRequestRateLimitMutex sync.RWMutex
	modelRequestRateLimitGroup = map[string][2]int{}
)

// ModelRequestRateLimitGroup2JSONString serializes per-group request-limit overrides to JSON.
func ModelRequestRateLimitGroup2JSONString() string {
	modelRequestRateLimitMutex.RLock()
	defer modelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(modelRequestRateLimitGroup)
	if err != nil {
		platformobservability.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateModelRequestRateLimitGroupByJSONString replaces per-group request-limit overrides from JSON.
func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	next := make(map[string][2]int)
	if err := json.Unmarshal([]byte(jsonStr), &next); err != nil {
		return err
	}

	modelRequestRateLimitMutex.Lock()
	defer modelRequestRateLimitMutex.Unlock()
	modelRequestRateLimitGroup = next
	return nil
}

// GetGroupRateLimit returns the configured total/success limits for one group.
func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	modelRequestRateLimitMutex.RLock()
	defer modelRequestRateLimitMutex.RUnlock()

	limits, found := modelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

// CheckModelRequestRateLimitGroup validates a JSON-encoded per-group request-limit map.
func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return fmt.Errorf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1])
		}
	}

	return nil
}

// SensitiveWordsToString serializes sensitive words into newline-separated text.
func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

// SensitiveWordsFromString replaces sensitive words from newline-separated text.
func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

// ShouldCheckPromptSensitive reports whether prompt-sensitive-word checks are enabled.
func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}
