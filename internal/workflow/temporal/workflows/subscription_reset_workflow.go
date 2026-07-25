package workflows

import (
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/workflow"
)

func SubscriptionResetWorkflow(ctx workflow.Context, input contracts.SubscriptionResetWorkflowInput) (*contracts.SubscriptionResetWorkflowOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions())

	if err := workflow.ExecuteActivity(ctx, contracts.ActivityFindResettableSubs, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCreateResetLedgerEntries, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityResetUsageProjection, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityPublishResetAuditEvents, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	return &contracts.SubscriptionResetWorkflowOutput{ResetStatus: "registered"}, nil
}
