package app

import (
	"fmt"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"strconv"
	"time"

	// ListAdminSubscriptionPlans returns all subscription plans for admin management.
	"gorm.io/gorm"
)

func ListAdminSubscriptionPlans() ([]SubscriptionPlanDTO, error) {
	var plans []commerceschema.SubscriptionPlan
	if err := platformdb.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		return nil, err
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, plan := range plans {
		result = append(result, SubscriptionPlanDTO{Plan: plan})
	}
	return result, nil
}

// CreateAdminSubscriptionPlan creates a new subscription plan from an admin payload.
func CreateAdminSubscriptionPlan(req AdminUpsertSubscriptionPlanRequest) (commerceschema.SubscriptionPlan, error) {
	req.Plan.Id = 0
	if err := validateSubscriptionPlanInput(&req.Plan); err != nil {
		return commerceschema.SubscriptionPlan{}, err
	}
	if err := platformdb.DB.Create(&req.Plan).Error; err != nil {
		return commerceschema.SubscriptionPlan{}, err
	}
	InvalidateSubscriptionPlanCache(req.Plan.Id)
	return req.Plan, nil
}

// UpdateAdminSubscriptionPlan updates a subscription plan with the admin payload.
func UpdateAdminSubscriptionPlan(planID int, req AdminUpsertSubscriptionPlanRequest) error {
	req.Plan.Id = planID
	if err := validateSubscriptionPlanInput(&req.Plan); err != nil {
		return err
	}

	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		updateMap := map[string]interface{}{
			"title":                      req.Plan.Title,
			"subtitle":                   req.Plan.Subtitle,
			"price_amount":               req.Plan.PriceAmount,
			"currency":                   req.Plan.Currency,
			"duration_unit":              req.Plan.DurationUnit,
			"duration_value":             req.Plan.DurationValue,
			"custom_seconds":             req.Plan.CustomSeconds,
			"enabled":                    req.Plan.Enabled,
			"internal_only":              req.Plan.InternalOnly,
			"sort_order":                 req.Plan.SortOrder,
			"stripe_price_id":            req.Plan.StripePriceId,
			"creem_product_id":           req.Plan.CreemProductId,
			"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
			"plan_type":                  req.Plan.PlanType,
			"fuel_enabled":               req.Plan.FuelEnabled,
			"fuel_unit_price":            req.Plan.FuelUnitPrice,
			"fuel_min_quota":             req.Plan.FuelMinQuota,
			"fuel_quota_step":            req.Plan.FuelQuotaStep,
			"total_amount":               req.Plan.TotalAmount,
			"period_amount":              req.Plan.PeriodAmount,
			"model_limits":               req.Plan.ModelLimits,
			"upgrade_group":              req.Plan.UpgradeGroup,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
			"updated_at":                 platformruntime.GetTimestamp(),
		}
		if err := tx.Model(&commerceschema.SubscriptionPlan{}).Where("id = ?", planID).Updates(updateMap).Error; err != nil {
			return err
		}
		InvalidateSubscriptionPlanCache(planID)
		return nil
	})
}

// UpdateAdminSubscriptionPlanStatus toggles a subscription plan's enabled status.
func UpdateAdminSubscriptionPlanStatus(planID int, enabled bool) error {
	if err := platformdb.DB.Model(&commerceschema.SubscriptionPlan{}).Where("id = ?", planID).Update("enabled", enabled).Error; err != nil {
		return err
	}
	InvalidateSubscriptionPlanCache(planID)
	return nil
}

// DeleteAdminSubscriptionPlan deletes a subscription plan when it has no active subscriptions.
func DeleteAdminSubscriptionPlan(planID int) (string, error) {
	if planID <= 0 {
		return "", errorsNew("invalid planId")
	}

	plan := &commerceschema.SubscriptionPlan{}
	if err := platformdb.DB.Where("id = ?", planID).First(plan).Error; err != nil {
		return "", err
	}

	now := platformruntime.GetTimestamp()
	var subscriptionCount int64
	if err := platformdb.DB.Model(&commerceschema.UserSubscription{}).
		Where("plan_id = ? AND status = ? AND end_time > ?", planID, "active", now).
		Count(&subscriptionCount).Error; err != nil {
		return "", err
	}
	if subscriptionCount > 0 {
		return "", errorsNew("cannot delete a plan that still has active subscriptions")
	}

	if err := platformdb.DB.Where("id = ?", planID).Delete(&commerceschema.SubscriptionPlan{}).Error; err != nil {
		return "", err
	}
	InvalidateSubscriptionPlanCache(planID)
	return "", nil
}

// BindAdminSubscription binds a plan directly to a user.
func BindAdminSubscription(req AdminBindSubscriptionRequest) (string, error) {
	return bindAdminSubscription(req.UserId, req.PlanId)
}

// ListAdminUserSubscriptions returns all subscriptions for a target user.
func ListAdminUserSubscriptions(userID int) ([]commercedomain.SubscriptionSummary, error) {
	return GetAllUserSubscriptions(userID)
}

// CreateAdminUserSubscription creates a subscription for the target user from the given plan.
func CreateAdminUserSubscription(userID int, req AdminCreateUserSubscriptionRequest) (string, error) {
	return bindAdminSubscription(userID, req.PlanId)
}

// UpdateAdminUserSubscription updates an existing user subscription.
func UpdateAdminUserSubscription(subscriptionID int, req AdminUpdateUserSubscriptionRequest) (string, error) {
	status, ok := normalizeAdminUserSubscriptionStatus(req.Status)
	if !ok {
		return "", errorsNew("invalid subscription status")
	}
	modelLimits, err := normalizeSubscriptionModelLimits(req.ModelLimits)
	if err != nil {
		return "", errorsNew("model_limits must be a valid JSON object")
	}
	return updateAdminUserSubscriptionRuntime(subscriptionID, adminUpdateUserSubscriptionRuntimeInput{
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Status:       status,
		AmountTotal:  req.AmountTotal,
		AmountUsed:   req.AmountUsed,
		PeriodAmount: req.PeriodAmount,
		PeriodUsed:   req.PeriodUsed,
		ModelLimits:  modelLimits,
	})
}

// InvalidateAdminUserSubscription cancels a user subscription immediately.
func InvalidateAdminUserSubscription(subscriptionID int) (string, error) {
	return invalidateAdminUserSubscriptionRuntime(subscriptionID)
}

// DeleteAdminUserSubscription deletes a user subscription permanently.
func DeleteAdminUserSubscription(subscriptionID int) (string, error) {
	return deleteAdminUserSubscriptionRuntime(subscriptionID)
}

// ResetAdminUserSubscriptionQuota resets subscription usage and records the admin action log.
func ResetAdminUserSubscriptionQuota(subscriptionID int, req AdminResetUserSubscriptionQuotaRequest, adminID int, adminName string) (map[string]any, error) {
	sub, err := resetAdminUserSubscriptionQuotaRuntime(subscriptionID, adminResetUserSubscriptionQuotaRuntimeInput{
		AdvanceResetTime: req.AdvanceResetTime,
	})
	if err != nil {
		return nil, err
	}
	adminInfo := map[string]interface{}{
		"admin_id":   adminID,
		"admin_name": adminName,
		"sub_id":     subscriptionID,
	}
	auditapp.RecordLogWithAdminInfo(sub.UserId, auditschema.LogTypeManage, "admin reset subscription quota", adminInfo)
	platformobservability.SysLog("admin reset subscription quota, sub_id=" + strconv.Itoa(subscriptionID))
	return map[string]any{
		"message":         "subscription quota reset",
		"subscription_id": sub.Id,
		"next_reset_time": sub.NextResetTime,
	}, nil
}

// ResetSubscriptionPeriodProjection resets the current subscription period from workflow
// orchestration. Ledger accounting is performed by the caller; this function updates the
// compatibility projection without clearing the subscription's cumulative consumption.
func ResetSubscriptionPeriodProjection(subscriptionID int) (*commerceschema.UserSubscription, error) {
	if subscriptionID <= 0 {
		return nil, errorsNew("invalid userSubscriptionId")
	}
	now := platformruntime.GetTimestamp()
	var updated commerceschema.UserSubscription
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		sub := &commerceschema.UserSubscription{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", subscriptionID).First(sub).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanRecordTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		if commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod) == commerceschema.SubscriptionResetNever {
			return errorsNew("subscription does not have a periodic quota")
		}
		if usesLegacySubscriptionPeriodicQuota(plan, sub) {
			sub.AmountUsed = 0
			if err := restoreSubscriptionLedgerBalanceAfterResetTx(tx, sub, fmt.Sprintf("period-projection:%d:%d", sub.Id, now)); err != nil {
				return err
			}
		} else {
			sub.PeriodUsed = 0
		}
		sub.ModelUsage = ""
		sub.LastResetTime = now
		sub.NextResetTime = calcNextSubscriptionResetTime(time.Unix(now, 0), plan, sub.EndTime)
		if err := tx.Save(sub).Error; err != nil {
			return err
		}
		updated = *sub
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func subscriptionAdminMessagePayload(message string) any {
	if message == "" {
		return nil
	}
	return map[string]any{"message": message}
}

func validateAdminUserSubscriptionRequest(req AdminUpdateUserSubscriptionRequest) error {
	switch {
	case req.StartTime <= 0 || req.EndTime <= 0 || req.EndTime <= req.StartTime:
		return errorsNew("invalid subscription time range")
	case req.AmountTotal < 0 || req.AmountUsed < 0 || req.PeriodAmount < 0 || req.PeriodUsed < 0:
		return errorsNew("quota values must be >= 0")
	case req.AmountTotal > 0 && req.AmountUsed > req.AmountTotal:
		return errorsNew("amount_used cannot exceed amount_total")
	case req.PeriodAmount > 0 && req.PeriodUsed > req.PeriodAmount:
		return errorsNew("period_used cannot exceed period_amount")
	default:
		return nil
	}
}

func errorsNew(message string) error {
	return fmt.Errorf("%s", message)
}
