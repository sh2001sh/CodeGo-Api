package activities

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
)

type BillingActivities struct{}

func parseSubscriptionID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid subscription id")
	}
	return id, nil
}

func (a *BillingActivities) CreateReservation(ctx context.Context, input contracts.RequestSettlementWorkflowInput) (*contracts.ReservationResult, error) {
	if strings.TrimSpace(input.ReservationID) != "" {
		var reservation billingschema.BillingReservation
		if err := platformdb.DB.WithContext(ctx).Where("reservation_id = ?", input.ReservationID).First(&reservation).Error; err != nil {
			return nil, err
		}
		if reservation.RequestID != input.RequestID || reservation.AccountID != input.AccountID {
			return nil, billingdomain.ErrLedgerConflict
		}
		return &contracts.ReservationResult{ReservationID: reservation.ReservationID}, nil
	}

	reservation, err := billingdomain.CreateReservation(billingdomain.CreateReservationParams{
		AccountID:      input.AccountID,
		RequestID:      input.RequestID,
		WorkflowID:     "request-settlement-" + input.RequestID,
		ReservedAmount: input.ReservedAmount,
		IdempotencyKey: "workflow:" + input.RequestID + ":reserve",
	})
	if err != nil {
		return nil, err
	}
	return &contracts.ReservationResult{ReservationID: reservation.ReservationID}, nil
}

func (a *BillingActivities) CreateSettlement(ctx context.Context, input contracts.RequestSettlementWorkflowInput) (*contracts.SettlementResult, error) {
	if strings.TrimSpace(input.SettlementID) != "" {
		var settlement billingschema.BillingSettlement
		if err := platformdb.DB.WithContext(ctx).Where("settlement_id = ?", input.SettlementID).First(&settlement).Error; err != nil {
			return nil, err
		}
		if settlement.ReservationID != input.ReservationID {
			return nil, billingdomain.ErrLedgerConflict
		}
		return &contracts.SettlementResult{SettlementID: settlement.SettlementID}, nil
	}

	if strings.TrimSpace(input.ReservationID) == "" {
		return nil, fmt.Errorf("reservation id is required to settle request")
	}
	settlement, err := billingdomain.SettleReservation(billingdomain.SettleReservationParams{
		ReservationID:   input.ReservationID,
		UsageEvidenceID: input.UsageEvidenceID,
		ActualAmount:    input.ActualAmount,
		IdempotencyKey:  "workflow:" + input.RequestID + ":settle",
	})
	if err != nil {
		return nil, err
	}
	return &contracts.SettlementResult{SettlementID: settlement.SettlementID}, nil
}

func (a *BillingActivities) RefundReference(ctx context.Context, input contracts.AsyncTaskWorkflowInput) error {
	if strings.TrimSpace(input.ReservationID) == "" {
		return nil
	}
	_, err := billingdomain.ReleaseReservation(billingdomain.ReleaseReservationParams{
		ReservationID:  input.ReservationID,
		IdempotencyKey: "workflow:task:" + input.PublicTaskID + ":release",
		ReasonCode:     "async_task_terminal_refund",
	})
	return err
}

func (a *BillingActivities) CreateResetLedgerEntries(ctx context.Context, input contracts.SubscriptionResetWorkflowInput) error {
	subscriptionID, err := parseSubscriptionID(input.SubscriptionID)
	if err != nil {
		return err
	}
	account, err := billingdomain.EnsureBillingAccount(billingdomain.EnsureAccountParams{
		AccountType: "subscription", OwnerType: "user_subscription", OwnerID: subscriptionID, QuotaUnit: "quota",
	})
	if err != nil {
		return err
	}
	_, err = billingdomain.RecordAdjustment(billingdomain.RecordAdjustmentParams{
		AccountID: account.AccountID, IdempotencyKey: "subscription-reset:" + input.SubscriptionID + ":" + input.ResetReason,
		ReasonCode: "subscription_period_reset", ReferenceType: "user_subscription", ReferenceID: input.SubscriptionID,
		OperatorType: "workflow", OperatorID: input.RequestedBy,
	})
	return err
}

func (a *BillingActivities) RefreshAccountSnapshot(ctx context.Context, accountID string) error {
	return billingapp.RebuildBalanceSnapshot(ctx, accountID)
}
