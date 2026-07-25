package encodingx

import (
	"bytes"
	"encoding/json"
	"io"
)

// Unmarshal decodes JSON bytes into the provided target value.
func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// UnmarshalString decodes a JSON string payload into the provided target value.
func UnmarshalString(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}

// DecodeJSON decodes JSON from a reader into the provided target value.
func DecodeJSON(reader io.Reader, v any) error {
	return json.NewDecoder(reader).Decode(v)
}

// Marshal encodes the provided value to JSON bytes.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// GetJSONType returns the top-level JSON token kind for a raw message.
func GetJSONType(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "unknown"
	}
	switch trimmed[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// JSONRawMessageToString returns decoded JSON strings and raw text for other JSON values.
func JSONRawMessageToString(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] != '"' {
		return string(trimmed)
	}

	var value string
	if err := Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	return value
}
