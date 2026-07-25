package app

import (
	"errors"
	"time"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

type SubscriptionFundingPreConsumeResult struct {
	UserSubscriptionID int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedAfter    int64
}

type SubscriptionFundingPlanInfo struct {
	PlanID    int
	PlanTitle string
}

type SubscriptionFundingHooks struct {
	PreConsume        func(requestID string, userID int, modelName string, amount int64) (*SubscriptionFundingPreConsumeResult, error)
	ReserveAdditional func(requestID string, subscriptionID int, modelName string, amount int64) error
	GetPlanInfo       func(userSubscriptionID int) (*SubscriptionFundingPlanInfo, error)
	PostConsumeDelta  func(subscriptionID int, modelName string, delta int64) error
	RefundPreConsume  func(requestID string) error
	SettleReservation func(requestID string, subscriptionID int, modelName string, actualAmount int64) error
}

var subscriptionFundingHooks SubscriptionFundingHooks

func RegisterSubscriptionFundingHooks(hooks SubscriptionFundingHooks) {
	subscriptionFundingHooks = hooks
}

func postSubscriptionUsageDelta(subscriptionID int, modelName string, delta int64) error {
	if delta == 0 {
		return nil
	}
	if subscriptionFundingHooks.PostConsumeDelta == nil {
		return errors.New("subscription funding settle hook is not registered")
	}
	return subscriptionFundingHooks.PostConsumeDelta(subscriptionID, modelName, delta)
}

type FundingSource interface {
	Source() string
	PreConsume(amount int) error
	Settle(delta int) error
	Refund() error
}

type ReservableFundingSource interface {
	ReserveAdditional(amount int64) error
}

type WalletFunding struct {
	userID   int
	consumed int
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := AdjustWalletQuota(w.userID, amount); err != nil {
		return err
	}
	w.consumed = amount
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	return AdjustWalletQuota(w.userID, delta)
}

func (w *WalletFunding) Refund() error {
	if w.consumed <= 0 {
		return nil
	}
	return AdjustWalletQuota(w.userID, -w.consumed)
}

type SubscriptionFunding struct {
	requestID       string
	userID          int
	modelName       string
	amount          int64
	subscriptionID  int
	preConsumed     int64
	AmountTotal     int64
	AmountUsedAfter int64
	PlanId          int
	PlanTitle       string
	reservationID   string
	accountID       string
	settlementID    string
}

func (s *SubscriptionFunding) Source() string { return BillingSourceSubscription }

func (s *SubscriptionFunding) PreConsume(_ int) error {
	if subscriptionFundingHooks.PreConsume == nil {
		return errors.New("subscription funding pre-consume hook is not registered")
	}
	res, err := subscriptionFundingHooks.PreConsume(s.requestID, s.userID, s.modelName, s.amount)
	if err != nil {
		return err
	}
	s.subscriptionID = res.UserSubscriptionID
	s.preConsumed = res.PreConsumed
	s.AmountTotal = res.AmountTotal
	s.AmountUsedAfter = res.AmountUsedAfter
	if reservation, err := findSubscriptionReservation(s.requestID); err != nil {
		return err
	} else if reservation != nil {
		s.reservationID = reservation.ReservationID
		s.accountID = reservation.AccountID
	}
	if subscriptionFundingHooks.GetPlanInfo != nil {
		if planInfo, err := subscriptionFundingHooks.GetPlanInfo(res.UserSubscriptionID); err == nil && planInfo != nil {
			s.PlanId = planInfo.PlanID
			s.PlanTitle = planInfo.PlanTitle
		}
	}
	return nil
}

func (s *SubscriptionFunding) Settle(delta int) error {
	actual := s.preConsumed + int64(delta)
	if actual < 0 {
		actual = 0
	}
	if s.reservationID != "" && subscriptionFundingHooks.SettleReservation != nil {
		if err := subscriptionFundingHooks.SettleReservation(s.requestID, s.subscriptionID, s.modelName, actual); err != nil {
			return err
		}
		return s.loadSettlement()
	}
	return postSubscriptionUsageDelta(s.subscriptionID, s.modelName, int64(delta))
}

func (s *SubscriptionFunding) ReservationID() string { return s.reservationID }

func (s *SubscriptionFunding) AccountID() string { return s.accountID }

func (s *SubscriptionFunding) SettlementID() string { return s.settlementID }

func (s *SubscriptionFunding) loadSettlement() error {
	if s.reservationID == "" {
		return errors.New("subscription ledger reservation is missing")
	}
	var settlement billingschema.BillingSettlement
	if err := platformdb.DB.Where("reservation_id = ?", s.reservationID).First(&settlement).Error; err != nil {
		return err
	}
	s.settlementID = settlement.SettlementID
	return nil
}

func (s *SubscriptionFunding) ReserveAdditional(amount int64) error {
	if amount <= 0 {
		return nil
	}
	if subscriptionFundingHooks.ReserveAdditional == nil {
		return errors.New("subscription additional reservation hook is not registered")
	}
	if err := subscriptionFundingHooks.ReserveAdditional(s.requestID, s.subscriptionID, s.modelName, amount); err != nil {
		return err
	}
	s.preConsumed += amount
	s.AmountUsedAfter += amount
	return nil
}

func (s *SubscriptionFunding) Refund() error {
	if s.preConsumed <= 0 {
		return nil
	}
	if subscriptionFundingHooks.RefundPreConsume == nil {
		return errors.New("subscription funding refund hook is not registered")
	}
	return refundWithRetry(func() error {
		return subscriptionFundingHooks.RefundPreConsume(s.requestID)
	})
}

func findSubscriptionReservation(requestID string) (*billingschema.BillingReservation, error) {
	var reservation billingschema.BillingReservation
	err := platformdb.DB.Where("idempotency_key = ?", "subscription:"+requestID+":reserve").First(&reservation).Error
	if err == nil {
		return &reservation, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func refundWithRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	const maxAttempts = 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		}
	}
	return lastErr
}
