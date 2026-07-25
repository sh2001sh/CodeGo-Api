package workflows

import (
	"time"

	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/workflow"
)

const asyncTaskPollInterval = 15 * time.Second

func AsyncTaskWorkflow(ctx workflow.Context, input contracts.AsyncTaskWorkflowInput) (*contracts.AsyncTaskWorkflowOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions())

	if err := workflow.ExecuteActivity(ctx, contracts.ActivityRecordTaskWorkflow, input).Get(ctx, nil); err != nil {
		return nil, err
	}
	var submitResult contracts.AsyncTaskSubmitResult
	if err := workflow.ExecuteActivity(ctx, contracts.ActivitySubmitAsyncTask, input).Get(ctx, &submitResult); err != nil {
		return nil, err
	}
	var (
		pollResult      contracts.AsyncTaskPollResult
		finalizeResult  contracts.AsyncTaskFinalizeResult
		finalizeLoaded  bool
		timeoutExceeded bool
	)
	for {
		if !input.TimeoutAt.IsZero() && !workflow.Now(ctx).Before(input.TimeoutAt) {
			timeoutExceeded = true
			break
		}
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityPollAsyncTaskStatus, input).Get(ctx, &pollResult); err != nil {
			return nil, err
		}
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityRecordTaskSnapshot, input).Get(ctx, nil); err != nil {
			return nil, err
		}
		if pollResult.Done {
			if err := workflow.ExecuteActivity(ctx, contracts.ActivityFinalizeTaskTerminalState, input).Get(ctx, &finalizeResult); err != nil {
				return nil, err
			}
			finalizeLoaded = true
			break
		}

		sleepFor := asyncTaskPollInterval
		if !input.TimeoutAt.IsZero() {
			remaining := input.TimeoutAt.Sub(workflow.Now(ctx))
			if remaining <= 0 {
				timeoutExceeded = true
				break
			}
			if remaining < sleepFor {
				sleepFor = remaining
			}
		}
		if sleepFor <= 0 {
			timeoutExceeded = true
			break
		}
		if err := workflow.Sleep(ctx, sleepFor); err != nil {
			return nil, err
		}
	}
	if timeoutExceeded && !finalizeLoaded {
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityFinalizeTaskTerminalState, input).Get(ctx, &finalizeResult); err != nil {
			return nil, err
		}
		finalizeLoaded = true
	}
	if finalizeLoaded && finalizeResult.SettlementStatus == "refunded" {
		if err := workflow.ExecuteActivity(ctx, contracts.ActivityRefundReference, input).Get(ctx, nil); err != nil {
			return nil, err
		}
	}
	if err := workflow.ExecuteActivity(ctx, contracts.ActivityProjectTaskResult, input).Get(ctx, nil); err != nil {
		return nil, err
	}

	output := &contracts.AsyncTaskWorkflowOutput{
		SettlementStatus: submitResult.ExternalTaskID,
	}
	if finalizeLoaded {
		output.TerminalState = finalizeResult.TerminalState
		output.SettlementStatus = finalizeResult.SettlementStatus
		output.ResultURL = finalizeResult.ResultURL
		output.FailureReason = finalizeResult.FailureReason
	} else {
		output.TerminalState = pollResult.TerminalState
		output.ResultURL = pollResult.ResultURL
		output.FailureReason = pollResult.FailureReason
	}
	return output, nil
}
