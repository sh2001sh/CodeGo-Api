package schema

import (
	"errors"
	"strings"

	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
)

const (
	SubscriptionPurchaseActionSubscribe = "subscribe"
	SubscriptionPurchaseActionRenew     = "renew"
	SubscriptionPurchaseActionUpgrade   = "upgrade"
	SubscriptionPurchaseActionDisabled  = "disabled"
)

const (
	SubscriptionPurchaseTypeNormal = "normal"
	SubscriptionPurchaseTypeFuel   = "subscription_fuel"
)

const (
	SubscriptionOrderFulfillmentPending   = "pending"
	SubscriptionOrderFulfillmentCompleted = "completed"
)

const (
	SubscriptionPlanTypeMonthly = "monthly"
	SubscriptionPlanTypeWeekly  = "weekly"
	SubscriptionPlanTypeDaily   = "daily"
	SubscriptionPlanTypeStarter = "starter"
)

// SubscriptionPlan is a purchasable subscription product.
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled      bool `json:"enabled" gorm:"default:true"`
	InternalOnly bool `json:"internal_only" gorm:"default:false;index"`
	SortOrder    int  `json:"sort_order" gorm:"type:int;default:0"`

	StripePriceId  string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`

	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	PlanType      string  `json:"plan_type" gorm:"type:varchar(20);default:'monthly';index"`
	FuelEnabled   bool    `json:"fuel_enabled" gorm:"default:false"`
	FuelUnitPrice float64 `json:"fuel_unit_price" gorm:"type:decimal(10,4);default:0"`
	FuelMinQuota  int64   `json:"fuel_min_quota" gorm:"type:bigint;default:0"`
	FuelQuotaStep int64   `json:"fuel_quota_step" gorm:"type:bigint;default:0"`

	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	TotalAmount  int64  `json:"total_amount" gorm:"type:bigint;not null;default:0"`
	PeriodAmount int64  `json:"period_amount" gorm:"type:bigint;not null;default:0"`
	ModelLimits  string `json:"model_limits" gorm:"type:text"`

	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(_ *gorm.DB) error {
	now := platformruntime.GetTimestamp()
	if strings.TrimSpace(p.PlanType) == "" {
		p.PlanType = inferSubscriptionPlanType(p)
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(_ *gorm.DB) error {
	p.UpdatedAt = platformruntime.GetTimestamp()
	return nil
}

func inferSubscriptionPlanType(plan *SubscriptionPlan) string {
	if plan == nil {
		return SubscriptionPlanTypeMonthly
	}
	title := strings.ToLower(strings.TrimSpace(plan.Title))
	if strings.Contains(title, "新人") || strings.Contains(title, "体验") || strings.Contains(title, "starter") {
		return SubscriptionPlanTypeStarter
	}
	switch plan.DurationUnit {
	case SubscriptionDurationDay:
		if plan.DurationValue >= 7 {
			return SubscriptionPlanTypeWeekly
		}
		return SubscriptionPlanTypeDaily
	case SubscriptionDurationMonth, SubscriptionDurationYear:
		return SubscriptionPlanTypeMonthly
	default:
		return normalizeSubscriptionPlanType(plan.PlanType)
	}
}

func normalizeSubscriptionPlanType(planType string) string {
	switch strings.TrimSpace(strings.ToLower(planType)) {
	case SubscriptionPlanTypeStarter:
		return SubscriptionPlanTypeStarter
	case SubscriptionPlanTypeWeekly:
		return SubscriptionPlanTypeWeekly
	case SubscriptionPlanTypeDaily:
		return SubscriptionPlanTypeDaily
	default:
		return SubscriptionPlanTypeMonthly
	}
}

// SubscriptionOrder records the payment transaction for a subscription purchase.
type SubscriptionOrder struct {
	Id                   int     `json:"id"`
	UserId               int     `json:"user_id" gorm:"index"`
	PlanId               int     `json:"plan_id" gorm:"index"`
	Money                float64 `json:"money"`
	TradeNo              string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod        string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider      string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	PurchaseType         string  `json:"purchase_type" gorm:"type:varchar(32);default:'normal';index"`
	TargetSubscriptionId int     `json:"target_subscription_id" gorm:"type:int;default:0;index"`
	FuelQuota            int64   `json:"fuel_quota" gorm:"type:bigint;default:0"`
	FuelUnitPrice        float64 `json:"fuel_unit_price" gorm:"type:decimal(10,4);default:0"`
	FuelExpiresAt        int64   `json:"fuel_expires_at" gorm:"type:bigint;default:0"`
	Status               string  `json:"status"`
	// FulfillmentStatus separates provider payment confirmation from benefits delivery.
	// Existing historical rows default to completed so migrations cannot re-grant benefits.
	FulfillmentStatus string `json:"fulfillment_status" gorm:"type:varchar(32);default:'completed';index"`
	CreateTime        int64  `json:"create_time"`
	CompleteTime      int64  `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

// UserSubscription is a user's active or historical subscription instance.
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal  int64  `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed   int64  `json:"amount_used" gorm:"type:bigint;not null;default:0"`
	PeriodAmount int64  `json:"period_amount" gorm:"type:bigint;not null;default:0"`
	PeriodUsed   int64  `json:"period_used" gorm:"type:bigint;not null;default:0"`
	ModelLimits  string `json:"model_limits" gorm:"type:text"`
	ModelUsage   string `json:"model_usage" gorm:"type:text"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"`
	Source    string `json:"source" gorm:"type:varchar(32);default:'order'"`

	LastResetTime int64  `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64  `json:"next_reset_time" gorm:"type:bigint;default:0;index"`
	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(_ *gorm.DB) error {
	now := platformruntime.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(_ *gorm.DB) error {
	s.UpdatedAt = platformruntime.GetTimestamp()
	return nil
}

// SubscriptionPreConsumeRecord makes pre-consume and refund operations idempotent.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);default:''"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(_ *gorm.DB) error {
	now := platformruntime.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(_ *gorm.DB) error {
	r.UpdatedAt = platformruntime.GetTimestamp()
	return nil
}
