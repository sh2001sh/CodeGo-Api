package activities

import (
	"context"
	"fmt"
	"strings"

	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/activity"
)

type TaskActivities struct{}

func (a *TaskActivities) SubmitAsyncTask(ctx context.Context, input contracts.AsyncTaskWorkflowInput) (*contracts.AsyncTaskSubmitResult, error) {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	if err := workflowdomain.UpdateTaskWorkflowFields(input.PublicTaskID, map[string]any{
		"status": taskWorkflowStatus(task),
	}); err != nil {
		return nil, err
	}
	return &contracts.AsyncTaskSubmitResult{ExternalTaskID: task.GetUpstreamTaskID()}, nil
}

func (a *TaskActivities) PollAsyncTaskStatus(ctx context.Context, input contracts.AsyncTaskWorkflowInput) (*contracts.AsyncTaskPollResult, error) {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	if workflowdomain.IsTaskTerminalStatus(task.Status) {
		return currentTaskPollResult(task), nil
	}

	if err := dispatchSingleTaskPoll(ctx, task); err != nil {
		return nil, err
	}
	refreshed, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	if err := workflowdomain.UpdateTaskWorkflowFields(input.PublicTaskID, map[string]any{
		"status":         taskWorkflowStatus(refreshed),
		"terminal_state": currentTaskPollResult(refreshed).TerminalState,
		"result_url":     refreshed.GetResultURL(),
		"result_meta":    taskResultMeta(refreshed),
	}); err != nil {
		return nil, err
	}
	return currentTaskPollResult(refreshed), nil
}

func (a *TaskActivities) RecordTaskWorkflow(ctx context.Context, input contracts.AsyncTaskWorkflowInput) error {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return err
	}
	return workflowdomain.UpsertTaskWorkflow(buildTaskWorkflowRecord(input, activity.GetInfo(ctx), task, "created"))
}

func (a *TaskActivities) RecordTaskSnapshot(ctx context.Context, input contracts.AsyncTaskWorkflowInput) error {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return err
	}
	record, err := workflowdomain.GetTaskWorkflowByPublicTaskID(input.PublicTaskID)
	if err != nil {
		return err
	}
	if record == nil || record.WorkflowID == "" {
		return fmt.Errorf("workflow record for task %q not found", input.PublicTaskID)
	}
	return workflowdomain.InsertTaskWorkflowSnapshot(&workflowschema.WorkflowTaskSnapshot{
		WorkflowID:       record.WorkflowID,
		ProviderState:    string(task.Status),
		ProviderProgress: taskProgressPercent(task.Progress),
		RawPayload:       taskResultMeta(task),
		ResultURL:        task.GetResultURL(),
		FailureReason:    task.FailReason,
	})
}

func (a *TaskActivities) FinalizeTaskTerminalState(ctx context.Context, input contracts.AsyncTaskWorkflowInput) (*contracts.AsyncTaskFinalizeResult, error) {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	task, err = finalizeTaskTimeout(ctx, task, taskTimeoutAt(input, task))
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task %q not found after finalize", input.PublicTaskID)
	}
	if !workflowdomain.IsTaskTerminalStatus(task.Status) {
		return nil, fmt.Errorf("task %q is not terminal", input.PublicTaskID)
	}

	record, err := workflowdomain.GetTaskWorkflowByPublicTaskID(input.PublicTaskID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.WorkflowID == "" {
		return nil, fmt.Errorf("workflow record for task %q not found", input.PublicTaskID)
	}

	finalize := &contracts.AsyncTaskFinalizeResult{
		TerminalState:    taskTerminalState(task),
		SettlementStatus: taskSettlementStatus(task),
		ResultURL:        task.GetResultURL(),
		FailureReason:    strings.TrimSpace(task.FailReason),
	}
	if err := workflowdomain.UpsertTaskTerminalResult(&workflowschema.WorkflowTaskTerminalResult{
		WorkflowID:       record.WorkflowID,
		TerminalState:    finalize.TerminalState,
		SettlementStatus: finalize.SettlementStatus,
		ResultURL:        finalize.ResultURL,
		ResultMeta:       taskResultMeta(task),
	}); err != nil {
		return nil, err
	}
	if err := workflowdomain.UpdateTaskWorkflowFields(input.PublicTaskID, map[string]any{
		"status":         finalize.TerminalState,
		"terminal_state": finalize.TerminalState,
		"result_url":     finalize.ResultURL,
		"result_meta":    taskResultMeta(task),
	}); err != nil {
		return nil, err
	}
	return finalize, nil
}

func (a *TaskActivities) ProjectTaskResult(ctx context.Context, input contracts.AsyncTaskWorkflowInput) error {
	task, err := loadTaskByWorkflowInput(input)
	if err != nil {
		return err
	}
	status := taskWorkflowStatus(task)
	if workflowdomain.IsTaskTerminalStatus(task.Status) {
		status = "projected"
	}
	return workflowdomain.UpdateTaskWorkflowFields(input.PublicTaskID, map[string]any{
		"status":         status,
		"terminal_state": taskTerminalState(task),
		"result_url":     task.GetResultURL(),
		"result_meta":    taskResultMeta(task),
	})
}
