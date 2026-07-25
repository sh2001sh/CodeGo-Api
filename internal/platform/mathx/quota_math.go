package mathx

import "math"

const (
	SafeMaxRequestTokens  = 1 << 20
	SafeMaxCandidateCount = 128
	SafeMaxToolCallCount  = 1024
)

func ClampNonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func SaturatingInt64ToInt(value int64) int {
	if value <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}

func SaturatingFloat64ToInt(value float64) int {
	if value <= 0 || math.IsNaN(value) {
		return 0
	}
	if math.IsInf(value, 1) {
		return int(^uint(0) >> 1)
	}
	maxInt := float64(int64(^uint(0) >> 1))
	if value >= maxInt {
		return SaturatingInt64ToInt(int64(maxInt))
	}
	return int(math.Round(value))
}

func SaturatingMulToInt(a float64, b ...float64) int {
	result := a
	for _, item := range b {
		result *= item
		if math.IsInf(result, 0) {
			return SaturatingInt64ToInt(int64(^uint(0) >> 1))
		}
	}
	return SaturatingFloat64ToInt(result)
}

func ValidatePositiveCount(value int64, max int64) bool {
	return value > 0 && value <= max
}

func ValidateOptionalUintWithinRange(value *uint, max uint) bool {
	if value == nil {
		return true
	}
	return *value <= max
}

func ValidateOptionalIntWithinRange(value *int, min int, max int) bool {
	if value == nil {
		return true
	}
	return *value >= min && *value <= max
}
