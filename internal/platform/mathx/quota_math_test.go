package mathx

import (
	"math"
	"testing"
)

func TestSaturatingInt64ToInt(t *testing.T) {
	if got := SaturatingInt64ToInt(-1); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := SaturatingInt64ToInt(42); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := SaturatingInt64ToInt(math.MaxInt64); got <= 0 {
		t.Fatalf("expected positive saturated value, got %d", got)
	}
}

func TestSaturatingFloat64ToInt(t *testing.T) {
	if got := SaturatingFloat64ToInt(math.NaN()); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := SaturatingFloat64ToInt(12.6); got != 13 {
		t.Fatalf("expected rounded 13, got %d", got)
	}
	if got := SaturatingFloat64ToInt(math.Inf(1)); got <= 0 {
		t.Fatalf("expected positive saturated value, got %d", got)
	}
}
