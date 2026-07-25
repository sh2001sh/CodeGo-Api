package vertex

import platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"

func GetModelRegion(other string, localModelName string) string {
	if platformtext.IsJsonObject(other) {
		m, err := platformtext.StrToMap(other)
		if err != nil {
			return other
		}
		if m[localModelName] != nil {
			return m[localModelName].(string)
		}
		if v, ok := m["default"]; ok {
			return v.(string)
		}
		return "global"
	}
	return other
}
