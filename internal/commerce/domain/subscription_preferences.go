package domain

import "strings"

// NormalizeBillingPreference clamps the billing preference to supported values.
func NormalizeBillingPreference(pref string) string {
	switch strings.TrimSpace(pref) {
	case "subscription_first", "wallet_first", "subscription_only", "wallet_only":
		return strings.TrimSpace(pref)
	default:
		return "subscription_first"
	}
}

// DefaultFundingSourceOrderFromBillingPreference derives the default funding order from a preference.
func DefaultFundingSourceOrderFromBillingPreference(pref string) []string {
	switch NormalizeBillingPreference(pref) {
	case "wallet_first":
		return []string{"wallet", "subscription"}
	case "subscription_only":
		return []string{"subscription"}
	case "wallet_only":
		return []string{"wallet"}
	default:
		return []string{"subscription", "wallet"}
	}
}

// NormalizeFundingSourceOrder removes invalid or duplicate funding sources.
func NormalizeFundingSourceOrder(order []string, pref string) []string {
	fallback := DefaultFundingSourceOrderFromBillingPreference(pref)
	if len(order) == 0 {
		return append([]string(nil), fallback...)
	}

	validSources := map[string]struct{}{
		"subscription": {},
		"wallet":       {},
	}
	seen := make(map[string]struct{}, len(order))
	result := make([]string, 0, len(order))
	for _, source := range order {
		normalized := strings.TrimSpace(source)
		if _, ok := validSources[normalized]; !ok {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return append([]string(nil), fallback...)
	}
	return result
}

// BillingPreferenceFromFundingSourceOrder projects a funding order back to the legacy preference enum.
func BillingPreferenceFromFundingSourceOrder(order []string) string {
	normalized := NormalizeFundingSourceOrder(order, "subscription_first")
	subscriptionIndex := -1
	walletIndex := -1
	for index, source := range normalized {
		switch source {
		case "subscription":
			subscriptionIndex = index
		case "wallet":
			walletIndex = index
		}
	}

	switch {
	case subscriptionIndex >= 0 && walletIndex >= 0:
		if subscriptionIndex < walletIndex {
			return "subscription_first"
		}
		return "wallet_first"
	case subscriptionIndex >= 0:
		return "subscription_only"
	case walletIndex >= 0:
		return "wallet_only"
	default:
		return "subscription_first"
	}
}

// NormalizePositiveIntSlice removes non-positive and duplicate identifiers while preserving order.
func NormalizePositiveIntSlice(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
