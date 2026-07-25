package textx

import "strconv"

// GetStringIfEmpty returns the fallback when the input string is empty.
func GetStringIfEmpty(str string, defaultValue string) string {
	if str == "" {
		return defaultValue
	}
	return str
}

// String2Int converts a string to int and returns zero on failure.
func String2Int(str string) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return num
}

// StringsContains reports whether the slice contains the target string.
func StringsContains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}
