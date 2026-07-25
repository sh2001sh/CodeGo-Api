package app

import (
	"errors"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"strings"

	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"gorm.io/gorm"
)

// AdminUpsertSubscriptionPlanRequest captures admin plan create/update payloads.
type AdminUpsertSubscriptionPlanRequest struct {
	Plan commerceschema.SubscriptionPlan `json:"plan"`
}

// AdminUpdateSubscriptionPlanStatusRequest captures admin plan status patches.
type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

// AdminBindSubscriptionRequest binds a plan to a user without a payment flow.
type AdminBindSubscriptionRequest struct {
	UserId int `json:"user_id"`
	PlanId int `json:"plan_id"`
}

// AdminCreateUserSubscriptionRequest creates a user subscription from a plan.
type AdminCreateUserSubscriptionRequest struct {
	PlanId int `json:"plan_id"`
}

// AdminUpdateUserSubscriptionRequest captures editable user subscription fields.
type AdminUpdateUserSubscriptionRequest struct {
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	Status       string `json:"status"`
	AmountTotal  int64  `json:"amount_total"`
	AmountUsed   int64  `json:"amount_used"`
	PeriodAmount int64  `json:"period_amount"`
	PeriodUsed   int64  `json:"period_used"`
	ModelLimits  string `json:"model_limits"`
}

type adminUpdateUserSubscriptionRuntimeInput struct {
	StartTime    int64
	EndTime      int64
	Status       string
	AmountTotal  int64
	AmountUsed   int64
	PeriodAmount int64
	PeriodUsed   int64
	ModelLimits  string
}

// AdminResetUserSubscriptionQuotaRequest captures reset-quota options.
type AdminResetUserSubscriptionQuotaRequest struct {
	AdvanceResetTime bool `json:"advance_reset_time"`
}

type adminResetUserSubscriptionQuotaRuntimeInput struct {
	AdvanceResetTime bool
}

func normalizeSubscriptionCurrency(currency string) string {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if normalized == "" {
		return "USD"
	}
	return normalized
}

func normalizeSubscriptionModelLimits(raw string) (string, error) {
	limits, err := commercedomain.ParseSubscriptionModelQuotaMap(raw)
	if err != nil {
		return "", err
	}
	return commercedomain.EncodeSubscriptionModelQuotaMap(limits)
}

func normalizeAdminUserSubscriptionStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case "active", "expired", "cancelled":
		return strings.TrimSpace(status), true
	default:
		return "", false
	}
}

func isMonthlySubscriptionPlan(plan *commerceschema.SubscriptionPlan) bool {
	if plan == nil {
		return false
	}
	return commercedomain.NormalizeSubscriptionPlanType(plan.PlanType) == commerceschema.SubscriptionPlanTypeMonthly ||
		(plan.DurationUnit == commerceschema.SubscriptionDurationMonth && plan.DurationValue == 1)
}

func validateSubscriptionPlanInput(plan *commerceschema.SubscriptionPlan) error {
	if plan == nil {
		return gorm.ErrInvalidData
	}
	if strings.TrimSpace(plan.Title) == "" {
		return errors.New("plan title is required")
	}
	if plan.PriceAmount < 0 || plan.PriceAmount > 9999 {
		return errors.New("plan price is invalid")
	}
	if plan.MaxPurchasePerUser < 0 {
		return errors.New("max_purchase_per_user must be >= 0")
	}
	plan.PlanType = commercedomain.NormalizeSubscriptionPlanType(plan.PlanType)
	if plan.PlanType == commerceschema.SubscriptionPlanTypeStarter && plan.MaxPurchasePerUser == 0 {
		plan.MaxPurchasePerUser = 1
	}
	if plan.FuelUnitPrice < 0 || plan.FuelMinQuota < 0 || plan.FuelQuotaStep < 0 {
		return errors.New("fuel settings must be >= 0")
	}
	if plan.FuelEnabled && (!isMonthlySubscriptionPlan(plan) || plan.FuelUnitPrice <= 0 || plan.FuelMinQuota <= 0 || plan.FuelQuotaStep <= 0) {
		return errors.New("enabled fuel requires a monthly plan, price, minimum quota, and quota step")
	}
	if plan.TotalAmount < 0 || plan.PeriodAmount < 0 {
		return errors.New("quota values must be >= 0")
	}
	plan.Currency = normalizeSubscriptionCurrency(plan.Currency)
	if plan.DurationUnit == "" {
		plan.DurationUnit = commerceschema.SubscriptionDurationMonth
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != commerceschema.SubscriptionDurationCustom {
		plan.DurationValue = 1
	}
	normalizedModelLimits, err := normalizeSubscriptionModelLimits(plan.ModelLimits)
	if err != nil {
		return errors.New("model_limits must be a valid JSON object")
	}
	plan.ModelLimits = normalizedModelLimits
	plan.UpgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
	if plan.UpgradeGroup != "" {
		if _, ok := gatewaystore.GetGroupRatioCopy()[plan.UpgradeGroup]; !ok {
			return errors.New("upgrade group does not exist")
		}
	}
	plan.QuotaResetPeriod = commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod)
	if plan.QuotaResetPeriod == commerceschema.SubscriptionResetCustom && plan.QuotaResetCustomSeconds <= 0 {
		return errors.New("quota_reset_custom_seconds must be > 0")
	}
	if isMonthlySubscriptionPlan(plan) && plan.QuotaResetPeriod == commerceschema.SubscriptionResetWeekly && plan.PeriodAmount > 0 {
		plan.TotalAmount = plan.PeriodAmount * 4
	}
	return nil
}
