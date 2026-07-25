package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	workflowapp "github.com/sh2001sh/CodeGo-Api/internal/workflow/app"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/sdk/activity"
)

const legacyTaskCutoffUnix = 1740182400

func loadTaskByWorkflowInput(input contracts.AsyncTaskWorkflowInput) (*workflowschema.Task, error) {
	task, err := workflowdomain.GetTaskByPublicTaskID(strings.TrimSpace(input.PublicTaskID))
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task %q not found", input.PublicTaskID)
	}
	return task, nil
}

func taskTimeoutAt(input contracts.AsyncTaskWorkflowInput, task *workflowschema.Task) time.Time {
	if !input.TimeoutAt.IsZero() {
		return input.TimeoutAt
	}
	if task == nil || task.SubmitTime <= 0 || constant.TaskTimeoutMinutes <= 0 {
		return time.Time{}
	}
	return time.Unix(task.SubmitTime, 0).Add(time.Duration(constant.TaskTimeoutMinutes) * time.Minute)
}

func currentTaskPollResult(task *workflowschema.Task) *contracts.AsyncTaskPollResult {
	if task == nil {
		return &contracts.AsyncTaskPollResult{}
	}
	result := &contracts.AsyncTaskPollResult{
		TerminalState: taskTerminalState(task),
		ResultURL:     task.GetResultURL(),
		FailureReason: task.FailReason,
	}
	result.Done = workflowdomain.IsTaskTerminalStatus(task.Status)
	return result
}

func taskTerminalState(task *workflowschema.Task) string {
	if task == nil {
		return ""
	}
	if workflowdomain.IsTaskSuccessStatus(task.Status) {
		return "succeeded"
	}
	if !workflowdomain.IsTaskFailureStatus(task.Status) {
		return ""
	}
	if strings.Contains(task.FailReason, "超时") {
		return "timeout"
	}
	return "failed"
}

func taskSettlementStatus(task *workflowschema.Task) string {
	if task == nil {
		return "pending"
	}
	switch task.Status {
	case workflowdomain.TaskStatusSuccess:
		return "settled"
	case workflowdomain.TaskStatusFailure:
		return "refunded"
	default:
		return "pending"
	}
}

func taskWorkflowStatus(task *workflowschema.Task) string {
	if task == nil {
		return "unknown"
	}
	switch task.Status {
	case workflowdomain.TaskStatusNotStart:
		return "created"
	case workflowdomain.TaskStatusSubmitted:
		return "submitted"
	case workflowdomain.TaskStatusQueued:
		return "queued"
	case workflowdomain.TaskStatusInProgress:
		return "running"
	case workflowdomain.TaskStatusSuccess:
		return "succeeded"
	case workflowdomain.TaskStatusFailure:
		return taskTerminalState(task)
	default:
		return strings.ToLower(string(task.Status))
	}
}

func taskProgressPercent(progress string) int {
	progress = strings.TrimSpace(strings.TrimSuffix(progress, "%"))
	if progress == "" {
		return 0
	}
	value, err := strconv.Atoi(progress)
	if err != nil {
		return 0
	}
	return value
}

func taskResultMeta(task *workflowschema.Task) json.RawMessage {
	if task == nil || len(task.Data) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(task.Data)
}

func finalizeTaskTimeout(ctx context.Context, task *workflowschema.Task, timeoutAt time.Time) (*workflowschema.Task, error) {
	if task == nil || !taskStatusIsPending(task) {
		return task, nil
	}
	if timeoutAt.IsZero() || time.Now().Before(timeoutAt) {
		return task, nil
	}

	snap := workflowdomain.TakeTaskSnapshot(task)
	decision := workflowdomain.ApplyTimeout(task, time.Now().Unix(), constant.TaskTimeoutMinutes, legacyTaskCutoffUnix)
	won, err := workflowdomain.UpdateTaskWithStatus(task, snap.Status)
	if err != nil {
		return nil, err
	}
	if !won {
		return workflowdomain.GetTaskByPublicTaskID(task.TaskID)
	}
	if decision.ShouldRefund {
		workflowapp.RecordTaskRefundForWorkflow(ctx, task, decision.Reason)
	}
	return task, nil
}

func taskStatusIsPending(task *workflowschema.Task) bool {
	return task != nil && !workflowdomain.IsTaskTerminalStatus(task.Status)
}

func buildTaskWorkflowRecord(input contracts.AsyncTaskWorkflowInput, info activity.Info, task *workflowschema.Task, status string) *workflowschema.WorkflowTaskWorkflow {
	timeoutAt := taskTimeoutAt(input, task)
	record := &workflowschema.WorkflowTaskWorkflow{
		PublicTaskID:       input.PublicTaskID,
		RequestID:          strings.TrimSpace(input.RequestID),
		AccountID:          strings.TrimSpace(input.AccountID),
		ProviderCode:       strings.TrimSpace(input.ProviderCode),
		ChannelID:          input.ChannelID,
		ReservationID:      strings.TrimSpace(input.ReservationID),
		TaskKind:           strings.TrimSpace(input.TaskKind),
		TemporalWorkflowID: info.WorkflowExecution.ID,
		TemporalRunID:      info.WorkflowExecution.RunID,
		Status:             status,
	}
	if record.RequestID == "" && task != nil {
		record.RequestID = strings.TrimSpace(task.PrivateData.RequestID)
	}
	if record.ProviderCode == "" && task != nil {
		record.ProviderCode = string(task.Platform)
	}
	if record.ChannelID == 0 && task != nil {
		record.ChannelID = int64(task.ChannelId)
	}
	if record.TaskKind == "" && task != nil {
		record.TaskKind = task.Action
	}
	if !timeoutAt.IsZero() {
		record.TimeoutAt = &timeoutAt
	}
	return record
}

func dispatchSingleTaskPoll(ctx context.Context, task *workflowschema.Task) error {
	return workflowapp.PollSingleTask(ctx, task)
}
