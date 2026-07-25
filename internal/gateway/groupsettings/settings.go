package groupsettings

import (
	"encoding/json"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"sync"
)

var (
	autoGroupsMu sync.RWMutex
	autoGroups   = []string{
		"default",
	}

	userUsableGroupsMutex sync.RWMutex
	userUsableGroups      = map[string]string{
		"default": "默认分组",
		"vip":     "vip分组",
	}
)

var DefaultUseAutoGroup = false

// ContainsAutoGroup reports whether the configured auto-group list contains the target group.
func ContainsAutoGroup(group string) bool {
	autoGroupsMu.RLock()
	defer autoGroupsMu.RUnlock()

	for _, autoGroup := range autoGroups {
		if autoGroup == group {
			return true
		}
	}
	return false
}

// UpdateAutoGroupsByJsonString replaces auto groups from a JSON array string.
func UpdateAutoGroupsByJsonString(jsonString string) error {
	next := make([]string, 0)
	if err := platformencoding.Unmarshal([]byte(jsonString), &next); err != nil {
		return err
	}

	autoGroupsMu.Lock()
	defer autoGroupsMu.Unlock()
	autoGroups = next
	return nil
}

// AutoGroups2JsonString serializes auto groups to JSON.
func AutoGroups2JsonString() string {
	autoGroupsMu.RLock()
	defer autoGroupsMu.RUnlock()

	jsonBytes, err := platformencoding.Marshal(autoGroups)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

// GetAutoGroups returns a copy of the configured auto groups.
func GetAutoGroups() []string {
	autoGroupsMu.RLock()
	defer autoGroupsMu.RUnlock()
	return append([]string(nil), autoGroups...)
}

// GetUserUsableGroupsCopy returns a copy of the user-usable group map.
func GetUserUsableGroupsCopy() map[string]string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	copyUserUsableGroups := make(map[string]string, len(userUsableGroups))
	for k, v := range userUsableGroups {
		copyUserUsableGroups[k] = v
	}
	return copyUserUsableGroups
}

// UserUsableGroups2JSONString serializes user-usable groups to JSON.
func UserUsableGroups2JSONString() string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	jsonBytes, err := json.Marshal(userUsableGroups)
	if err != nil {
		platformobservability.SysLog("error marshalling user groups: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateUserUsableGroupsByJSONString replaces user-usable groups from a JSON object string.
func UpdateUserUsableGroupsByJSONString(jsonStr string) error {
	next := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonStr), &next); err != nil {
		return err
	}

	userUsableGroupsMutex.Lock()
	defer userUsableGroupsMutex.Unlock()
	userUsableGroups = next
	return nil
}

// GetUsableGroupDescription returns the configured label for one usable group.
func GetUsableGroupDescription(groupName string) string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	if desc, ok := userUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}
