package activities

import (
	"context"
	"strings"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"gorm.io/gorm"
)

type GatewayActivities struct{}

func (a *GatewayActivities) CreateRequestExecution(ctx context.Context, input contracts.RequestSettlementWorkflowInput) (*contracts.RequestExecutionResult, error) {
	var existing gatewayschema.RequestExecution
	err := platformdb.DB.WithContext(ctx).Where("request_id = ?", input.RequestID).First(&existing).Error
	if err == nil {
		return &contracts.RequestExecutionResult{ExecutionID: existing.ExecutionID}, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	routePlanID := strings.TrimSpace(input.RoutePlanID)
	if routePlanID == "" {
		routePlanID = input.RequestID
	}
	returnResult := &contracts.RequestExecutionResult{}
	err = platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan := gatewayschema.GatewayRoutePlan{RoutePlanID: routePlanID, RequestID: input.RequestID, TraceID: input.TraceID, Status: "recorded"}
		if err := tx.Where("request_id = ?", input.RequestID).FirstOrCreate(&plan).Error; err != nil {
			return err
		}
		execution := gatewayschema.RequestExecution{
			RequestID: input.RequestID, UserID: input.UserID, TokenID: input.TokenID, AccountID: input.AccountID,
			ReservationID: input.ReservationID, SettlementID: input.SettlementID, RoutePlanID: plan.RoutePlanID, TraceID: input.TraceID,
			Status: gatewayschema.RequestExecutionStatusRecorded, ActualAmount: input.ActualAmount,
		}
		if err := tx.Create(&execution).Error; err != nil {
			return err
		}
		attempt := gatewayschema.ExecutionAttempt{ExecutionID: execution.ExecutionID, TraceID: input.TraceID, AttemptNo: 1, Status: "provider_completed"}
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		returnResult.ExecutionID = execution.ExecutionID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returnResult, nil
}

func (a *GatewayActivities) ExecuteProviderRequest(ctx context.Context, input contracts.RequestSettlementWorkflowInput) error {
	// The gateway executes the provider request synchronously to preserve HTTP and streaming semantics.
	// This workflow receives only its durable completion projection.
	return platformdb.DB.WithContext(ctx).Model(&gatewayschema.RequestExecution{}).
		Where("request_id = ?", input.RequestID).
		Update("status", gatewayschema.RequestExecutionStatusProviderComplete).Error
}

func (a *GatewayActivities) CollectUsageEvidence(ctx context.Context, input contracts.RequestSettlementWorkflowInput) (*contracts.UsageEvidenceResult, error) {
	evidenceID := strings.TrimSpace(input.UsageEvidenceID)
	if evidenceID == "" {
		evidenceID = input.RequestID
	}
	var execution gatewayschema.RequestExecution
	if err := platformdb.DB.WithContext(ctx).Where("request_id = ?", input.RequestID).First(&execution).Error; err != nil {
		return nil, err
	}
	evidence := gatewayschema.UsageEvidence{UsageEvidenceID: evidenceID, ExecutionID: execution.ExecutionID, RequestID: input.RequestID, TraceID: input.TraceID, ActualAmount: input.ActualAmount}
	if err := platformdb.DB.WithContext(ctx).Where("request_id = ?", input.RequestID).FirstOrCreate(&evidence).Error; err != nil {
		return nil, err
	}
	if err := platformdb.DB.WithContext(ctx).Model(&gatewayschema.RequestExecution{}).Where("execution_id = ?", execution.ExecutionID).
		Update("usage_evidence_id", evidence.UsageEvidenceID).Error; err != nil {
		return nil, err
	}
	return &contracts.UsageEvidenceResult{UsageEvidenceID: evidenceID}, nil
}

func (a *GatewayActivities) PublishRequestSettledEvent(ctx context.Context, input contracts.RequestSettlementWorkflowInput) error {
	if err := platformdb.DB.WithContext(ctx).Model(&gatewayschema.RequestExecution{}).Where("request_id = ?", input.RequestID).
		Updates(map[string]any{"status": gatewayschema.RequestExecutionStatusSettled, "settlement_id": input.SettlementID, "actual_amount": input.ActualAmount}).Error; err != nil {
		return err
	}
	return nil
}
