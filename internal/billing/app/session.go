package app

import (
	"errors"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"net/http"
	"strings"
	"sync"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type BillingSession struct {
	relayInfo        *relaycommon.RelayInfo
	funding          FundingSource
	preConsumedQuota int
	tokenConsumed    int
	extraReserved    int
	trusted          bool
	fundingSettled   bool
	settled          bool
	refunded         bool
	mu               sync.Mutex
}

func (s *BillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if actualQuota < 0 {
		actualQuota = 0
	}
	if s.settled {
		return nil
	}
	if s.trusted && s.preConsumedQuota == 0 && actualQuota > 0 {
		if err := s.reserveTrustedSettlement(actualQuota); err != nil {
			return err
		}
	}

	delta := actualQuota - s.preConsumedQuota
	if !s.fundingSettled {
		if err := s.funding.Settle(delta); err != nil {
			return err
		}
		s.fundingSettled = true
	}

	var tokenErr error
	if !s.relayInfo.IsPlayground {
		tokenErr = AdjustTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, delta)
		if tokenErr != nil {
			platformobservability.SysLog(fmt.Sprintf(
				"error adjusting token quota after funding settled (userId=%d, tokenId=%d, delta=%d): %s",
				s.relayInfo.UserId,
				s.relayInfo.TokenId,
				delta,
				tokenErr.Error(),
			))
		}
	}

	if s.funding.Source() == BillingSourceSubscription {
		s.relayInfo.SubscriptionPostDelta += int64(delta)
	}

	s.settled = true
	return tokenErr
}

func (s *BillingSession) reserveTrustedSettlement(actualQuota int) error {
	if err := s.funding.PreConsume(actualQuota); err != nil {
		return err
	}
	if !s.relayInfo.IsPlayground {
		if err := PreConsumeTokenQuota(s.relayInfo, actualQuota); err != nil {
			if refundErr := s.funding.Refund(); refundErr != nil {
				return errors.Join(err, fmt.Errorf("refund trusted funding reservation: %w", refundErr))
			}
			return err
		}
	}
	s.preConsumedQuota = actualQuota
	s.tokenConsumed = actualQuota
	s.syncRelayInfo()
	return nil
}

func (s *BillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	if s.settled || s.refunded || !s.needsRefundLocked() {
		s.mu.Unlock()
		return
	}
	s.refunded = true
	s.mu.Unlock()

	logger.LogInfo(c, fmt.Sprintf(
		"user %d request failed, refund pre-consume (token_quota=%s, funding=%s)",
		s.relayInfo.UserId,
		logger.FormatQuota(s.tokenConsumed),
		s.funding.Source(),
	))

	tokenID := s.relayInfo.TokenId
	tokenKey := s.relayInfo.TokenKey
	isPlayground := s.relayInfo.IsPlayground
	tokenConsumed := s.tokenConsumed
	funding := s.funding

	gopool.Go(func() {
		if err := funding.Refund(); err != nil {
			platformobservability.SysLog("error refunding billing source: " + err.Error())
		}

		if tokenConsumed > 0 && !isPlayground {
			if err := AdjustTokenQuota(tokenID, tokenKey, -tokenConsumed); err != nil {
				platformobservability.SysLog("error refunding token quota: " + err.Error())
			}
		}
	})
}

func (s *BillingSession) RefundSync(c *gin.Context) error {
	s.mu.Lock()
	if s.settled || s.refunded || !s.needsRefundLocked() {
		s.mu.Unlock()
		return nil
	}
	s.refunded = true
	s.mu.Unlock()

	logger.LogInfo(c, fmt.Sprintf(
		"user %d request failed, refund pre-consume (token_quota=%s, funding=%s)",
		s.relayInfo.UserId,
		logger.FormatQuota(s.tokenConsumed),
		s.funding.Source(),
	))

	tokenID := s.relayInfo.TokenId
	tokenKey := s.relayInfo.TokenKey
	isPlayground := s.relayInfo.IsPlayground
	tokenConsumed := s.tokenConsumed
	funding := s.funding

	var errs []error
	if err := funding.Refund(); err != nil {
		errs = append(errs, fmt.Errorf("refund funding source: %w", err))
	}

	if tokenConsumed > 0 && !isPlayground {
		if err := AdjustTokenQuota(tokenID, tokenKey, -tokenConsumed); err != nil {
			errs = append(errs, fmt.Errorf("refund token quota: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (s *BillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsRefundLocked()
}

func (s *BillingSession) needsRefundLocked() bool {
	if s.settled || s.refunded || s.fundingSettled {
		return false
	}
	if s.tokenConsumed > 0 {
		return true
	}
	if sub, ok := s.funding.(*SubscriptionFunding); ok && sub.preConsumed > 0 {
		return true
	}
	return false
}

func (s *BillingSession) GetPreConsumedQuota() int {
	return s.preConsumedQuota
}

func (s *BillingSession) Reserve(targetQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if targetQuota < 0 {
		targetQuota = 0
	}
	if s.settled || s.refunded || s.trusted || targetQuota <= s.preConsumedQuota {
		return nil
	}

	delta := targetQuota - s.preConsumedQuota
	if delta <= 0 {
		return nil
	}

	if err := s.reserveFunding(delta); err != nil {
		return err
	}
	if err := s.reserveToken(delta); err != nil {
		s.rollbackFundingReserve(delta)
		return err
	}

	s.preConsumedQuota += delta
	s.tokenConsumed += delta
	s.extraReserved += delta
	s.syncRelayInfo()
	return nil
}

func (s *BillingSession) preConsume(c *gin.Context, quota int) *types.APIError {
	effectiveQuota := quota

	if s.shouldTrust(c) {
		s.trusted = true
		effectiveQuota = 0
		logger.LogInfo(c, fmt.Sprintf(
			"user %d has enough trusted quota, skipping pre-consume (funding=%s)",
			s.relayInfo.UserId,
			s.funding.Source(),
		))
	} else if effectiveQuota > 0 {
		logger.LogInfo(c, fmt.Sprintf(
			"user %d pre-consume %s (funding=%s)",
			s.relayInfo.UserId,
			logger.FormatQuota(effectiveQuota),
			s.funding.Source(),
		))
	}

	if effectiveQuota > 0 {
		if err := PreConsumeTokenQuota(s.relayInfo, effectiveQuota); err != nil {
			return types.NewErrorWithStatusCode(
				err,
				types.ErrorCodePreConsumeTokenQuotaFailed,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			)
		}
		s.tokenConsumed = effectiveQuota
	}

	if err := s.funding.PreConsume(effectiveQuota); err != nil {
		if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
			if rollbackErr := AdjustTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, -s.tokenConsumed); rollbackErr != nil {
				platformobservability.SysLog(fmt.Sprintf(
					"error rolling back token quota (userId=%d, tokenId=%d, amount=%d, fundingErr=%s): %s",
					s.relayInfo.UserId,
					s.relayInfo.TokenId,
					s.tokenConsumed,
					err.Error(),
					rollbackErr.Error(),
				))
			}
			s.tokenConsumed = 0
		}

		if insufficientErr := newFundingInsufficientError(s.funding.Source(), err); insufficientErr != nil {
			return insufficientErr
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "no active subscription") || strings.Contains(errMsg, "subscription quota insufficient") {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("subscription quota insufficient or unavailable: %s", errMsg),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			)
		}

		return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}

	s.preConsumedQuota = effectiveQuota
	s.syncRelayInfo()
	return nil
}

func newFundingInsufficientError(source string, err error) *types.APIError {
	var message string
	switch {
	case errors.Is(err, billingdomain.ErrInsufficientBalance):
		switch source {
		case BillingSourceSubscription:
			message = "subscription quota insufficient or unavailable"
		default:
			message = "wallet quota insufficient"
		}
	default:
		return nil
	}

	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s: %s", message, err.Error()),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func (s *BillingSession) reserveFunding(delta int) error {
	if funding, ok := s.funding.(ReservableFundingSource); ok {
		return funding.ReserveAdditional(int64(delta))
	}
	switch funding := s.funding.(type) {
	case *WalletFunding:
		if err := AdjustWalletQuota(funding.userID, delta); err != nil {
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		funding.consumed += delta
		return nil
	case *SubscriptionFunding:
		return nil
	default:
		return types.NewError(
			fmt.Errorf("unsupported funding source: %s", s.funding.Source()),
			types.ErrorCodeUpdateDataError,
			types.ErrOptionWithSkipRetry(),
		)
	}
}

func (s *BillingSession) rollbackFundingReserve(delta int) {
	if _, ok := s.funding.(ReservableFundingSource); ok {
		return
	}
	switch funding := s.funding.(type) {
	case *WalletFunding:
		if err := AdjustWalletQuota(funding.userID, -delta); err != nil {
			platformobservability.SysLog("error rolling back wallet funding reserve: " + err.Error())
		} else {
			funding.consumed -= delta
		}
	}
}

func (s *BillingSession) reserveToken(delta int) error {
	if delta <= 0 || s.relayInfo.IsPlayground {
		return nil
	}
	if err := PreConsumeTokenQuota(s.relayInfo, delta); err != nil {
		return types.NewErrorWithStatusCode(
			err,
			types.ErrorCodePreConsumeTokenQuotaFailed,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return nil
}

func (s *BillingSession) shouldTrust(c *gin.Context) bool {
	if s.relayInfo.ForcePreConsume {
		return false
	}

	trustQuota := platformruntime.GetTrustQuota()
	if trustQuota <= 0 {
		return false
	}

	tokenTrusted := s.relayInfo.TokenUnlimited
	if !tokenTrusted {
		tokenQuota := c.GetInt("token_quota")
		tokenTrusted = tokenQuota > trustQuota
	}
	if !tokenTrusted {
		return false
	}

	switch s.funding.Source() {
	case BillingSourceWallet:
		return s.relayInfo.UserQuota > trustQuota
	case BillingSourceSubscription:
		return false
	default:
		return false
	}
}

func (s *BillingSession) syncRelayInfo() {
	info := s.relayInfo
	info.FinalPreConsumedQuota = s.preConsumedQuota
	info.BillingSource = s.funding.Source()

	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		info.SubscriptionId = sub.subscriptionID
		info.SubscriptionPreConsumed = sub.preConsumed + int64(s.extraReserved)
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = sub.AmountTotal
		info.SubscriptionAmountUsedAfterPreConsume = sub.AmountUsedAfter + int64(s.extraReserved)
		info.SubscriptionPlanId = sub.PlanId
		info.SubscriptionPlanTitle = sub.PlanTitle
	} else {
		info.SubscriptionId = 0
		info.SubscriptionPreConsumed = 0
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = 0
		info.SubscriptionAmountUsedAfterPreConsume = 0
		info.SubscriptionPlanId = 0
		info.SubscriptionPlanTitle = ""
	}
}

func NewBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.APIError) {
	if relayInfo == nil {
		return nil, types.NewError(
			fmt.Errorf("relayInfo is nil"),
			types.ErrorCodeInvalidRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	pref := commercedomain.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference)
	fundingSourceOrder := commercedomain.NormalizeFundingSourceOrder(
		relayInfo.UserSetting.FundingSourceOrder,
		pref,
	)

	tryWallet := func() (*BillingSession, *types.APIError) {
		userQuota, err := GetUserWalletQuota(relayInfo.UserId)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if userQuota <= 0 || userQuota-preConsumedQuota < 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("站内余额不足, 当前余额: %s, 本次所需: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			)
		}
		relayInfo.UserQuota = userQuota
		funding, err := NewLedgerRelayFunding(relayInfo.UserId, relayInfo.RequestId, BillingSourceWallet)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		session := &BillingSession{
			relayInfo: relayInfo,
			funding:   funding,
		}
		if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	trySubscription := func() (*BillingSession, *types.APIError) {
		subConsume := int64(preConsumedQuota)
		if subConsume <= 0 {
			subConsume = 1
		}

		session := &BillingSession{
			relayInfo: relayInfo,
			funding: &SubscriptionFunding{
				requestID: relayInfo.RequestId,
				userID:    relayInfo.UserId,
				modelName: relayInfo.OriginModelName,
				amount:    subConsume,
			},
		}
		if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	var lastInsufficientErr *types.APIError
	for _, source := range fundingSourceOrder {
		var (
			session *BillingSession
			apiErr  *types.APIError
		)

		switch source {
		case BillingSourceSubscription:
			session, apiErr = trySubscription()
		case BillingSourceWallet:
			session, apiErr = tryWallet()
		default:
			continue
		}

		if apiErr != nil {
			if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				lastInsufficientErr = apiErr
				continue
			}
			return nil, apiErr
		}
		if session != nil {
			return session, nil
		}
	}

	if lastInsufficientErr != nil {
		return nil, lastInsufficientErr
	}
	return nil, types.NewErrorWithStatusCode(
		fmt.Errorf("no available funding source"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}
