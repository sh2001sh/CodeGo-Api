package workflows

import (
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/workflow"
)

func RequestSettlementWorkflow(ctx workflow.Context, input contracts.RequestSettlementWorkflowInput) (*contracts.RequestSettlementWorkflowOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions())

	var execution contracts.RequestExecutionResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCreateRequestExecution, input).Get(ctx, &execution); err != nil {
		return nil, err
	}
	var reservation contracts.ReservationResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCreateReservation, input).Get(ctx, &reservation); err != nil {
		return nil, err
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityExecuteProviderRequest, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	var usageEvidence contracts.UsageEvidenceResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCollectUsageEvidence, input).Get(ctx, &usageEvidence); err != nil {
		return nil, err
	}
	var settlement contracts.SettlementResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCreateSettlement, input).Get(ctx, &settlement); err != nil {
		return nil, err
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityPublishRequestSettled, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	return &contracts.RequestSettlementWorkflowOutput{
		ExecutionStatus: "registered",
		ReservationID:   reservation.ReservationID,
		SettlementID:    settlement.SettlementID,
		UsageEvidenceID: usageEvidence.UsageEvidenceID,
	}, nil
}
