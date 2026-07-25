package app

import (
	"errors"
	"fmt"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	"strconv"
)

type AmountRequest struct {
	Amount int64 `json:"amount"`
}

type AdminCompleteTopupRequest struct {
	TradeNo string `json:"trade_no"`
}

func BuildTopUpInfo(userID int) map[string]any {
	complianceConfirmed := commercestore.IsPaymentComplianceConfirmed()
	return map[string]any{
		"enable_stripe_topup":              IsStripeTopUpEnabled(),
		"enable_creem_topup":               IsCreemTopUpEnabled(),
		"enable_redemption":                complianceConfirmed,
		"payment_compliance_confirmed":     complianceConfirmed,
		"payment_compliance_terms_version": commercestore.CurrentComplianceTermsVersion,
		"creem_products":                   commercestore.CreemProducts,
		"min_topup":                        commercestore.MinTopUp,
		"stripe_min_topup":                 commercestore.StripeMinTopUp,
		"amount_options":                   commercestore.GetPaymentSetting().AmountOptions,
		"discount":                         commercestore.GetPaymentSetting().AmountDiscount,
		"topup_link":                       platformconfig.TopUpLink,
	}
}

func QuoteTopUpAmount(userID int, req AmountRequest) (string, error) {
	minTopup := GetTopupMinAmount()
	if req.Amount < minTopup {
		return "", fmt.Errorf("充值数量不能小于 %d", GetMinTopup())
	}

	group, err := loadCommerceUserGroup(userID, true)
	if err != nil {
		return "", errors.New("获取用户分组失败")
	}

	payMoney := GetTopupPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		return "", errors.New("充值金额过低")
	}
	return strconv.FormatFloat(payMoney, 'f', 2, 64), nil
}

func ListUserTopUps(userID int, keyword string, pageInfo *platformpagination.PageInfo) (*platformpagination.PageInfo, error) {
	var (
		topups []*commerceschema.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = SearchUserTopUps(userID, keyword, pageInfo)
	} else {
		topups, total, err = GetUserTopUps(userID, pageInfo)
	}
	if err != nil {
		return nil, err
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	return pageInfo, nil
}

func ListAllTopUps(keyword string, pageInfo *platformpagination.PageInfo) (*platformpagination.PageInfo, error) {
	var (
		topups []*commerceschema.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = SearchAllTopUps(keyword, pageInfo)
	} else {
		topups, total, err = GetAllTopUps(pageInfo)
	}
	if err != nil {
		return nil, err
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	return pageInfo, nil
}

func CompleteTopUpByAdmin(tradeNo string, callerIP string) error {
	if tradeNo == "" {
		return errors.New("参数错误")
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	return ManualCompleteTopUp(tradeNo, callerIP)
}
