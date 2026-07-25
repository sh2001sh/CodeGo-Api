package encodingx

import (
	"fmt"
	"strconv"
)

// Interface2String converts common scalar values to their string representation.
func Interface2String(inter any) string {
	switch value := inter.(type) {
	case string:
		return value
	case int:
		return fmt.Sprintf("%d", value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		if value {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

// Any2Type converts arbitrary decoded data into the requested target type through JSON round-tripping.
func Any2Type[T any](data any) (T, error) {
	var zero T
	bytes, err := Marshal(data)
	if err != nil {
		return zero, err
	}
	var res T
	if err := Unmarshal(bytes, &res); err != nil {
		return zero, err
	}
	return res, nil
}
