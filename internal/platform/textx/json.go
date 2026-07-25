package textx

import (
	"encoding/base64"
	"encoding/json"

	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
)

// MapToJsonStr marshals a map into a JSON string and returns an empty string on failure.
func MapToJsonStr(m map[string]interface{}) string {
	bytes, err := platformencoding.Marshal(m)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// StrToMap unmarshals a JSON object string into a map.
func StrToMap(str string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	if err := platformencoding.UnmarshalString(str, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// StrToJsonArray unmarshals a JSON array string into a slice.
func StrToJsonArray(str string) ([]interface{}, error) {
	var js []interface{}
	if err := json.Unmarshal([]byte(str), &js); err != nil {
		return nil, err
	}
	return js, nil
}

// IsJsonArray reports whether the provided string is a valid JSON array.
func IsJsonArray(str string) bool {
	var js []interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// IsJsonObject reports whether the provided string is a valid JSON object.
func IsJsonObject(str string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// EncodeBase64 encodes the provided string using standard base64 encoding.
func EncodeBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// GetJsonString marshals any value to JSON and returns an empty string when the value is nil.
func GetJsonString(data any) string {
	if data == nil {
		return ""
	}
	b, _ := platformencoding.Marshal(data)
	return string(b)
}
