package app

import (
	"errors"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"strings"
	"unicode/utf8"
)

var (
	ErrRedemptionIDInvalid               = errors.New("无效的兑换码 ID")
	ErrRedemptionNameLengthInvalid       = errors.New("兑换码名称长度不合法")
	ErrRedemptionCountPositiveRequired   = errors.New("兑换码数量必须大于 0")
	ErrRedemptionCountMaxExceeded        = errors.New("兑换码数量不能超过 100")
	ErrRedemptionExpireTimeInvalid       = errors.New("兑换码过期时间无效")
	ErrRedemptionPayloadEmpty            = errors.New("redemption payload is empty")
	ErrRedemptionQuotaRequired           = errors.New("quota redemption requires quota greater than 0")
	ErrRedemptionSubscriptionPlanInvalid = errors.New("subscription redemption requires a valid plan")
	ErrRedemptionSubscriptionPlanMissing = errors.New("subscription plan not found")
)

// RedemptionCreateResult contains newly generated redemption keys.
type RedemptionCreateResult struct {
	Keys []string
}

// IsPaymentComplianceConfirmed reports whether payment compliance is currently satisfied.
func IsPaymentComplianceConfirmed() bool {
	return commercestore.IsPaymentComplianceConfirmed()
}

// ListRedemptions returns a paginated redemption list.
func ListRedemptions(offset int, limit int) ([]*commerceschema.Redemption, int64, error) {
	return listRedemptionRecords(offset, limit)
}

// SearchRedemptions returns a paginated redemption search result.
func SearchRedemptions(keyword string, offset int, limit int) ([]*commerceschema.Redemption, int64, error) {
	return searchRedemptionRecords(keyword, offset, limit)
}

// GetRedemption returns a redemption by ID.
func GetRedemption(id int) (*commerceschema.Redemption, error) {
	if id <= 0 {
		return nil, ErrRedemptionIDInvalid
	}
	return loadRedemptionRecord(id)
}

// CreateRedemption validates the request and creates redemption codes.
func CreateRedemption(actorUserID int, redemption commerceschema.Redemption) (*RedemptionCreateResult, error) {
	if err := validateCreateRedemptionInput(&redemption); err != nil {
		return nil, err
	}

	keys := make([]string, 0, redemption.Count)
	for i := 0; i < redemption.Count; i++ {
		cleanRedemption := commerceschema.Redemption{
			UserId:      actorUserID,
			Name:        redemption.Name,
			Key:         platformruntime.GetUUID(),
			CreatedTime: platformruntime.GetTimestamp(),
			RedeemType:  redemption.RedeemType,
			Quota:       redemption.Quota,
			PlanId:      redemption.PlanId,
			PlanTitle:   redemption.PlanTitle,
			ExpiredTime: redemption.ExpiredTime,
		}
		if err := createRedemptionRecord(&cleanRedemption); err != nil {
			return &RedemptionCreateResult{Keys: keys}, err
		}
		keys = append(keys, cleanRedemption.Key)
	}
	return &RedemptionCreateResult{Keys: keys}, nil
}

// UpdateRedemption validates and updates a redemption entry.
func UpdateRedemption(redemption commerceschema.Redemption, statusOnly bool) (*commerceschema.Redemption, error) {
	cleanRedemption, err := GetRedemption(redemption.Id)
	if err != nil {
		return nil, err
	}

	if !statusOnly {
		if err := validateRedemptionName(redemption.Name); err != nil {
			return nil, err
		}
		if err := validateExpiredTime(redemption.ExpiredTime); err != nil {
			return nil, err
		}
		cleanRedemption.Name = strings.TrimSpace(redemption.Name)
		cleanRedemption.RedeemType = redemption.RedeemType
		cleanRedemption.Quota = redemption.Quota
		cleanRedemption.PlanId = redemption.PlanId
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
		if err := prepareRedemptionForWrite(cleanRedemption); err != nil {
			return nil, err
		}
	}
	if statusOnly {
		cleanRedemption.Status = redemption.Status
	}
	if err := updateRedemptionRecord(cleanRedemption, statusOnly); err != nil {
		return nil, err
	}
	return cleanRedemption, nil
}

// DeleteRedemption deletes one redemption by ID.
func DeleteRedemption(id int) error {
	if id <= 0 {
		return ErrRedemptionIDInvalid
	}
	return deleteRedemptionRecord(id)
}

// DeleteInvalidRedemptions deletes used, disabled, and expired redemption codes.
func DeleteInvalidRedemptions() (int64, error) {
	now := platformruntime.GetTimestamp()
	return deleteInvalidRedemptionRecords(now)
}

func validateCreateRedemptionInput(redemption *commerceschema.Redemption) error {
	if err := validateRedemptionName(redemption.Name); err != nil {
		return err
	}
	if redemption.Count <= 0 {
		return ErrRedemptionCountPositiveRequired
	}
	if redemption.Count > 100 {
		return ErrRedemptionCountMaxExceeded
	}
	if err := validateExpiredTime(redemption.ExpiredTime); err != nil {
		return err
	}
	return prepareRedemptionForWrite(redemption)
}

func validateRedemptionName(name string) error {
	trimmed := strings.TrimSpace(name)
	if utf8.RuneCountInString(trimmed) == 0 || utf8.RuneCountInString(trimmed) > 20 {
		return ErrRedemptionNameLengthInvalid
	}
	return nil
}

func validateExpiredTime(expired int64) error {
	if expired != 0 && expired < platformruntime.GetTimestamp() {
		return ErrRedemptionExpireTimeInvalid
	}
	return nil
}

func prepareRedemptionForWrite(redemption *commerceschema.Redemption) error {
	if redemption == nil {
		return ErrRedemptionPayloadEmpty
	}

	redemption.Name = strings.TrimSpace(redemption.Name)
	redemption.RedeemType = commercedomain.NormalizeRedemptionType(redemption.RedeemType)
	switch redemption.RedeemType {
	case commerceschema.RedemptionTypeSubscription:
		if redemption.PlanId <= 0 {
			return ErrRedemptionSubscriptionPlanInvalid
		}
		plan, err := commerceapp.GetSubscriptionPlanByID(redemption.PlanId)
		if err != nil {
			return ErrRedemptionSubscriptionPlanMissing
		}
		redemption.PlanTitle = plan.Title
		redemption.Quota = 0
	default:
		if redemption.Quota <= 0 {
			return ErrRedemptionQuotaRequired
		}
		redemption.PlanId = 0
		redemption.PlanTitle = ""
	}
	return nil
}
