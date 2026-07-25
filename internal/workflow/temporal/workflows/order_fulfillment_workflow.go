package workflows

import (
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/workflow"
)

func OrderFulfillmentWorkflow(ctx workflow.Context, input contracts.OrderFulfillmentWorkflowInput) (*contracts.OrderFulfillmentWorkflowOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions())

	if err := workflow.ExecuteActivity(ctx, contracts.ActivityCreateOrderRecord, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	var validation contracts.OrderCallbackValidationResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityValidatePaymentCallback, input).Get(ctx, &validation); err != nil {
		return nil, err
	}
	if validation.Valid {
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityMarkOrderPaid, input).Get(ctx, nil); err != nil {
			return nil, err
		}
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityGrantOrderBenefits, input).Get(ctx, nil); err != nil {
			return nil, err
		}
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityPublishOrderPaidEvent, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	return &contracts.OrderFulfillmentWorkflowOutput{
		OrderStatus:   "registered",
		BenefitStatus: "registered",
	}, nil
}
