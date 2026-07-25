package schema

import (
	"errors"
	"fmt"
	"strings"

	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
)

type SubscriptionModelQuotaMap map[string]int64

func normalizeSubscriptionModelQuotaMap(input map[string]int64) SubscriptionModelQuotaMap {
	result := make(SubscriptionModelQuotaMap)
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || value <= 0 {
			continue
		}
		result[trimmedKey] = value
	}
	return result
}

func parseSubscriptionModelQuotaMap(raw string) (SubscriptionModelQuotaMap, error) {
	if strings.TrimSpace(raw) == "" {
		return SubscriptionModelQuotaMap{}, nil
	}
	result := make(SubscriptionModelQuotaMap)
	if err := platformencoding.UnmarshalString(raw, &result); err != nil {
		return nil, err
	}
	return normalizeSubscriptionModelQuotaMap(result), nil
}

func encodeSubscriptionModelQuotaMap(input map[string]int64) (string, error) {
	normalized := normalizeSubscriptionModelQuotaMap(input)
	if len(normalized) == 0 {
		return "", nil
	}
	raw, err := platformencoding.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func mustParseSubscriptionModelQuotaMap(raw string) SubscriptionModelQuotaMap {
	result, err := parseSubscriptionModelQuotaMap(raw)
	if err != nil {
		return SubscriptionModelQuotaMap{}
	}
	return result
}

func (p *SubscriptionPlan) GetModelLimitsMap() SubscriptionModelQuotaMap {
	return mustParseSubscriptionModelQuotaMap(p.ModelLimits)
}

func (p *SubscriptionPlan) SetModelLimitsMap(input map[string]int64) error {
	raw, err := encodeSubscriptionModelQuotaMap(input)
	if err != nil {
		return err
	}
	p.ModelLimits = raw
	return nil
}

func (s *UserSubscription) GetModelLimitsMap() SubscriptionModelQuotaMap {
	return mustParseSubscriptionModelQuotaMap(s.ModelLimits)
}

func (s *UserSubscription) SetModelLimitsMap(input map[string]int64) error {
	raw, err := encodeSubscriptionModelQuotaMap(input)
	if err != nil {
		return err
	}
	s.ModelLimits = raw
	return nil
}

func (s *UserSubscription) GetModelUsageMap() SubscriptionModelQuotaMap {
	return mustParseSubscriptionModelQuotaMap(s.ModelUsage)
}

func (s *UserSubscription) SetModelUsageMap(input map[string]int64) error {
	raw, err := encodeSubscriptionModelQuotaMap(input)
	if err != nil {
		return err
	}
	s.ModelUsage = raw
	return nil
}

func usesLegacyPeriodicQuota(plan *SubscriptionPlan, sub *UserSubscription) bool {
	if plan == nil || sub == nil || normalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return false
	}
	return sub.PeriodAmount <= 0 && sub.AmountTotal > 0
}

func getSubscriptionPeriodAmount(plan *SubscriptionPlan, sub *UserSubscription) int64 {
	if sub != nil && sub.PeriodAmount > 0 {
		return sub.PeriodAmount
	}
	if usesLegacyPeriodicQuota(plan, sub) && sub != nil {
		return sub.AmountTotal
	}
	if plan != nil && plan.PeriodAmount > 0 {
		return plan.PeriodAmount
	}
	return 0
}

// ApplyUsageDelta applies validated quota usage to a subscription instance.
func ApplyUsageDelta(plan *SubscriptionPlan, sub *UserSubscription, modelName string, delta int64) error {
	if sub == nil {
		return errors.New("subscription is nil")
	}
	if delta == 0 {
		return nil
	}

	legacyPeriodicQuota := usesLegacyPeriodicQuota(plan, sub)
	newAmountUsed := sub.AmountUsed + delta
	if newAmountUsed < 0 {
		newAmountUsed = 0
	}
	if sub.AmountTotal > 0 && newAmountUsed > sub.AmountTotal {
		return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newAmountUsed, sub.AmountTotal)
	}
	sub.AmountUsed = newAmountUsed

	if !legacyPeriodicQuota {
		periodAmount := getSubscriptionPeriodAmount(plan, sub)
		if periodAmount > 0 {
			newPeriodUsed := sub.PeriodUsed + delta
			if newPeriodUsed < 0 {
				newPeriodUsed = 0
			}
			if newPeriodUsed > periodAmount {
				return fmt.Errorf("subscription period quota exceeded, used=%d period=%d", newPeriodUsed, periodAmount)
			}
			sub.PeriodUsed = newPeriodUsed
		}
	}

	trimmedModelName := strings.TrimSpace(modelName)
	if trimmedModelName == "" {
		return nil
	}
	limits := sub.GetModelLimitsMap()
	limit, ok := limits[trimmedModelName]
	if !ok || limit <= 0 {
		return nil
	}
	usage := sub.GetModelUsageMap()
	newUsage := usage[trimmedModelName] + delta
	if newUsage < 0 {
		newUsage = 0
	}
	if newUsage > limit {
		return fmt.Errorf("subscription model quota exceeded, model=%s used=%d limit=%d", trimmedModelName, newUsage, limit)
	}
	if newUsage == 0 {
		delete(usage, trimmedModelName)
	} else {
		usage[trimmedModelName] = newUsage
	}
	return sub.SetModelUsageMap(usage)
}

func normalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}
