package domain

import "testing"

func TestNormalizeFundingSourceOrder(t *testing.T) {
	tests := []struct {
		name     string
		order    []string
		pref     string
		expected []string
	}{
		{
			name:     "falls back to legacy preference when empty",
			order:    nil,
			pref:     "wallet_first",
			expected: []string{"wallet", "subscription"},
		},
		{
			name:     "keeps requested order",
			order:    []string{"subscription", "wallet"},
			pref:     "subscription_first",
			expected: []string{"subscription", "wallet"},
		},
		{
			name:     "supports wallet first order",
			order:    []string{"wallet", "subscription"},
			pref:     "wallet_first",
			expected: []string{"wallet", "subscription"},
		},
		{
			name:     "supports wallet only mode",
			order:    []string{"wallet"},
			pref:     "wallet_only",
			expected: []string{"wallet"},
		},
		{
			name:     "rejects empty after filtering",
			order:    []string{},
			pref:     "subscription_only",
			expected: []string{"subscription"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := NormalizeFundingSourceOrder(test.order, test.pref)
			if len(actual) != len(test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, actual)
			}
			for index := range actual {
				if actual[index] != test.expected[index] {
					t.Fatalf("expected %v, got %v", test.expected, actual)
				}
			}
		})
	}
}

func TestBillingPreferenceFromFundingSourceOrder(t *testing.T) {
	tests := []struct {
		name     string
		order    []string
		expected string
	}{
		{
			name:     "projects subscription first order",
			order:    []string{"subscription", "wallet"},
			expected: "subscription_first",
		},
		{
			name:     "projects wallet first order",
			order:    []string{"wallet", "subscription"},
			expected: "wallet_first",
		},
		{
			name:     "projects subscription only order",
			order:    []string{"subscription"},
			expected: "subscription_only",
		},
		{
			name:     "projects wallet only order",
			order:    []string{"wallet"},
			expected: "wallet_only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := BillingPreferenceFromFundingSourceOrder(test.order)
			if actual != test.expected {
				t.Fatalf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestNormalizePositiveIntSlice(t *testing.T) {
	actual := NormalizePositiveIntSlice([]int{0, -1, 5, 3, 5, 2, 3})
	expected := []int{5, 3, 2}
	if len(actual) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("expected %v, got %v", expected, actual)
		}
	}
}
