package app

import (
	"errors"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samber/hot"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/cachex"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

const (
	subscriptionPlanCacheNamespace = "codego-api:subscription_plan:v1"
)

var (
	subscriptionPlanCacheOnce sync.Once
	subscriptionPlanCache     *cachex.HybridCache[commerceschema.SubscriptionPlan]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := platformconfig.GetEnvOrDefaultInt("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := platformconfig.GetEnvOrDefaultInt("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[commerceschema.SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[commerceschema.SubscriptionPlan](cachex.HybridCacheConfig[commerceschema.SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     platformcache.RDB,
			RedisEnabled: func() bool {
				return platformcache.RedisEnabled && platformcache.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[commerceschema.SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, commerceschema.SubscriptionPlan] {
				return hot.NewHotCache[string, commerceschema.SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func subscriptionPlanCacheKey(planID int) string {
	if planID <= 0 {
		return ""
	}
	return strconv.Itoa(planID)
}

// InvalidateSubscriptionPlanCache clears the cached snapshot for a plan.
func InvalidateSubscriptionPlanCache(planID int) {
	key := subscriptionPlanCacheKey(planID)
	if key == "" {
		return
	}
	_, _ = getSubscriptionPlanCache().DeleteMany([]string{key})
}

// GetSubscriptionPlanByID loads a subscription plan by identifier.
func GetSubscriptionPlanByID(planID int) (*commerceschema.SubscriptionPlan, error) {
	return getSubscriptionPlanRecordTx(nil, planID)
}

// GetSubscriptionPlanInfoByUserSubscriptionID returns the plan identity used
// by billing logs without exposing the full subscription or plan record.
func GetSubscriptionPlanInfoByUserSubscriptionID(userSubscriptionID int) (*commercedomain.SubscriptionPlanInfo, error) {
	if userSubscriptionID <= 0 {
		return nil, errors.New("invalid user subscription id")
	}

	var subscription commerceschema.UserSubscription
	if err := platformdb.DB.Select("id, plan_id").Where("id = ?", userSubscriptionID).First(&subscription).Error; err != nil {
		return nil, err
	}
	plan, err := GetSubscriptionPlanByID(subscription.PlanId)
	if err != nil {
		return nil, err
	}
	return &commercedomain.SubscriptionPlanInfo{PlanId: plan.Id, PlanTitle: plan.Title}, nil
}

func getSubscriptionPlanRecordTx(tx *gorm.DB, planID int) (*commerceschema.SubscriptionPlan, error) {
	if planID <= 0 {
		return nil, errors.New("invalid plan id")
	}

	key := subscriptionPlanCacheKey(planID)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			return &cached, nil
		}
	}

	plan := &commerceschema.SubscriptionPlan{}
	query := platformdb.DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", planID).First(plan).Error; err != nil {
		return nil, err
	}
	if key != "" {
		_ = getSubscriptionPlanCache().SetWithTTL(key, *plan, subscriptionPlanCacheTTL())
	}
	return plan, nil
}

// GetSubscriptionOrderByTradeNo loads a subscription order by trade number.
func GetSubscriptionOrderByTradeNo(tradeNo string) *commerceschema.SubscriptionOrder {
	if strings.TrimSpace(tradeNo) == "" {
		return nil
	}

	order := &commerceschema.SubscriptionOrder{}
	if err := platformdb.DB.Where("trade_no = ?", tradeNo).First(order).Error; err != nil {
		return nil
	}
	return order
}

// GetSubscriptionOrderByTradeNoForUser loads a user's subscription order by trade number.
func GetSubscriptionOrderByTradeNoForUser(tradeNo string, userID int) (*commerceschema.SubscriptionOrder, error) {
	if strings.TrimSpace(tradeNo) == "" || userID <= 0 {
		return nil, errors.New("invalid tradeNo or userId")
	}

	order := &commerceschema.SubscriptionOrder{}
	if err := platformdb.DB.Where("trade_no = ? AND user_id = ?", tradeNo, userID).First(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// CountUserSubscriptionsByPlan counts all subscriptions a user has purchased for a plan.
func CountUserSubscriptionsByPlan(userID int, planID int) (int64, error) {
	if userID <= 0 || planID <= 0 {
		return 0, errors.New("invalid userId or planId")
	}

	var count int64
	if err := platformdb.DB.Model(&commerceschema.UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userID, planID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetAllActiveUserSubscriptions returns ordered active subscriptions for a user.
func GetAllActiveUserSubscriptions(userID int) ([]commercedomain.SubscriptionSummary, error) {
	if userID <= 0 {
		return nil, errors.New("invalid userId")
	}

	now := platformruntime.GetTimestamp()
	var subs []commerceschema.UserSubscription
	if err := platformdb.DB.Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error; err != nil {
		return nil, err
	}

	ordered, err := orderActiveUserSubscriptions(userID, subs)
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(ordered), nil
}

// GetAllUserSubscriptions returns every subscription snapshot for a user.
func GetAllUserSubscriptions(userID int) ([]commercedomain.SubscriptionSummary, error) {
	if userID <= 0 {
		return nil, errors.New("invalid userId")
	}

	var subs []commerceschema.UserSubscription
	if err := platformdb.DB.Where("user_id = ?", userID).
		Order("end_time desc, id desc").
		Find(&subs).Error; err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

func buildSubscriptionSummaries(subs []commerceschema.UserSubscription) []commercedomain.SubscriptionSummary {
	if len(subs) == 0 {
		return []commercedomain.SubscriptionSummary{}
	}

	result := make([]commercedomain.SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		result = append(result, commercedomain.SubscriptionSummary{
			Subscription: &subCopy,
		})
	}
	return result
}

func orderActiveUserSubscriptions(userID int, subs []commerceschema.UserSubscription) ([]commerceschema.UserSubscription, error) {
	return orderActiveUserSubscriptionsTx(nil, userID, subs)
}

func orderActiveUserSubscriptionsTx(tx *gorm.DB, userID int, subs []commerceschema.UserSubscription) ([]commerceschema.UserSubscription, error) {
	if len(subs) == 0 {
		return []commerceschema.UserSubscription{}, nil
	}

	setting, err := identitystore.LoadUserSetting(userID, false)
	if err != nil {
		return orderActiveUserSubscriptionsWithExplicitOrderTx(tx, subs, nil)
	}
	return orderActiveUserSubscriptionsWithExplicitOrderTx(tx, subs, commercedomain.NormalizePositiveIntSlice(setting.SubscriptionOrderIds))
}

func orderActiveUserSubscriptionsWithExplicitOrder(subs []commerceschema.UserSubscription, explicitOrder []int) ([]commerceschema.UserSubscription, error) {
	return orderActiveUserSubscriptionsWithExplicitOrderTx(nil, subs, explicitOrder)
}

func orderActiveUserSubscriptionsWithExplicitOrderTx(tx *gorm.DB, subs []commerceschema.UserSubscription, explicitOrder []int) ([]commerceschema.UserSubscription, error) {
	if len(subs) == 0 {
		return []commerceschema.UserSubscription{}, nil
	}

	explicitRank := make(map[int]int, len(explicitOrder))
	for index, id := range explicitOrder {
		explicitRank[id] = index
	}

	planMap := make(map[int]*commerceschema.SubscriptionPlan, len(subs))
	for _, sub := range subs {
		if _, ok := planMap[sub.PlanId]; ok {
			continue
		}
		plan, err := getSubscriptionPlanRecordTx(tx, sub.PlanId)
		if err == nil && plan != nil {
			planMap[sub.PlanId] = plan
		}
	}

	ordered := append([]commerceschema.UserSubscription(nil), subs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]

		leftRank, leftHasRank := explicitRank[left.Id]
		rightRank, rightHasRank := explicitRank[right.Id]
		switch {
		case leftHasRank && rightHasRank && leftRank != rightRank:
			return leftRank < rightRank
		case leftHasRank != rightHasRank:
			return leftHasRank
		}

		leftBucket := 1
		if commercedomain.IsSubscriptionDayPassPlan(planMap[left.PlanId]) {
			leftBucket = 0
		}
		rightBucket := 1
		if commercedomain.IsSubscriptionDayPassPlan(planMap[right.PlanId]) {
			rightBucket = 0
		}
		if leftBucket != rightBucket {
			return leftBucket < rightBucket
		}

		if left.EndTime != right.EndTime {
			return left.EndTime > right.EndTime
		}
		return left.Id > right.Id
	})
	return ordered, nil
}

func isManagedSubscriptionPlan(plan *commerceschema.SubscriptionPlan) bool {
	return plan != nil && !commercedomain.IsSubscriptionDayPassPlan(plan)
}

func compareSubscriptionPlanTier(left *commerceschema.SubscriptionPlan, right *commerceschema.SubscriptionPlan) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	if left.PriceAmount < right.PriceAmount {
		return -1
	}
	if left.PriceAmount > right.PriceAmount {
		return 1
	}
	if left.TotalAmount < right.TotalAmount {
		return -1
	}
	if left.TotalAmount > right.TotalAmount {
		return 1
	}
	if left.PeriodAmount < right.PeriodAmount {
		return -1
	}
	if left.PeriodAmount > right.PeriodAmount {
		return 1
	}
	if left.Id < right.Id {
		return -1
	}
	if left.Id > right.Id {
		return 1
	}
	return 0
}

func hasMeaningfulSubscriptionQuotaRemaining(sub *commerceschema.UserSubscription) bool {
	if sub == nil {
		return false
	}
	remainingQuota := sub.AmountTotal - sub.AmountUsed
	if remainingQuota <= 0 {
		return false
	}
	return remainingQuota > quotaUnitsFromUSD(0.01)
}

func quotaUnitsFromUSD(amount float64) int64 {
	if amount <= 0 {
		return 0
	}
	return int64(math.Round(amount * platformruntime.QuotaPerUnit))
}
