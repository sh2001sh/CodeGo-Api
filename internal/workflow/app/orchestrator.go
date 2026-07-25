package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"
)

type Orchestrator struct {
	Temporal temporalclient.Client
}

// NewOrchestrator builds the workflow app facade that will own Temporal-backed orchestration entrypoints.
func NewOrchestrator(client temporalclient.Client) *Orchestrator {
	return &Orchestrator{Temporal: client}
}

// Enabled reports whether a Temporal client has been attached to the workflow app orchestrator.
func (o *Orchestrator) Enabled() bool {
	return o != nil && o.Temporal != nil
}

var (
	defaultTemporalClient     temporalclient.Client
	defaultTemporalClientErr  error
	defaultTemporalClientOnce sync.Once
)

func StartAsyncTaskWorkflow(ctx context.Context, task *workflowschema.Task) error {
	if task == nil {
		return nil
	}

	input := buildAsyncTaskWorkflowInput(task)
	client, err := defaultAsyncTaskTemporalClient()
	if err != nil {
		_ = workflowdomain.UpsertTaskWorkflow(asyncTaskWorkflowRecord(task, "", "", "orchestration_failed"))
		return err
	}

	options := temporalclient.StartWorkflowOptions{
		ID:                       "async-task-" + task.TaskID,
		TaskQueue:                platformconfig.GetEnvOrDefaultString("TEMPORAL_TASK_QUEUE_TASKS", "workflow-tasks"),
		WorkflowExecutionTimeout: 24 * time.Hour,
		WorkflowRunTimeout:       24 * time.Hour,
		WorkflowTaskTimeout:      15 * time.Second,
	}
	execution, err := client.ExecuteWorkflow(ctx, options, contracts.WorkflowAsyncTask, input)
	if err != nil {
		if _, alreadyStarted := err.(*serviceerror.WorkflowExecutionAlreadyStarted); alreadyStarted {
			return workflowdomain.UpsertTaskWorkflow(asyncTaskWorkflowRecord(task, options.ID, "", "scheduled"))
		}
		_ = workflowdomain.UpsertTaskWorkflow(asyncTaskWorkflowRecord(task, options.ID, "", "orchestration_failed"))
		return err
	}

	return workflowdomain.UpsertTaskWorkflow(asyncTaskWorkflowRecord(task, execution.GetID(), execution.GetRunID(), "scheduled"))
}

func buildAsyncTaskWorkflowInput(task *workflowschema.Task) contracts.AsyncTaskWorkflowInput {
	input := contracts.AsyncTaskWorkflowInput{
		WorkflowVersion: "v1",
		PublicTaskID:    task.TaskID,
		RequestID:       task.PrivateData.RequestID,
		ProviderCode:    string(task.Platform),
		ChannelID:       int64(task.ChannelId),
		TaskKind:        task.Action,
		SubmitPayload:   task.Data,
	}
	if task.SubmitTime > 0 && constant.TaskTimeoutMinutes > 0 {
		input.TimeoutAt = time.Unix(task.SubmitTime, 0).Add(time.Duration(constant.TaskTimeoutMinutes) * time.Minute)
	}
	return input
}

func asyncTaskWorkflowRecord(task *workflowschema.Task, workflowID string, runID string, status string) *workflowschema.WorkflowTaskWorkflow {
	record := &workflowschema.WorkflowTaskWorkflow{
		PublicTaskID:       task.TaskID,
		RequestID:          task.PrivateData.RequestID,
		ProviderCode:       string(task.Platform),
		ChannelID:          int64(task.ChannelId),
		TaskKind:           task.Action,
		TemporalWorkflowID: workflowID,
		TemporalRunID:      runID,
		Status:             status,
		ResultMeta:         task.Data,
	}
	if task.GetResultURL() != "" {
		record.ResultURL = task.GetResultURL()
	}
	if task.SubmitTime > 0 && constant.TaskTimeoutMinutes > 0 {
		timeoutAt := time.Unix(task.SubmitTime, 0).Add(time.Duration(constant.TaskTimeoutMinutes) * time.Minute)
		record.TimeoutAt = &timeoutAt
	}
	return record
}

func defaultAsyncTaskTemporalClient() (temporalclient.Client, error) {
	defaultTemporalClientOnce.Do(func() {
		hostPort := platformconfig.GetEnvOrDefaultString("TEMPORAL_HOSTPORT", "")
		if hostPort == "" {
			defaultTemporalClientErr = fmt.Errorf("TEMPORAL_HOSTPORT is required for async task workflow orchestration")
			return
		}
		defaultTemporalClient, defaultTemporalClientErr = temporalclient.Dial(temporalclient.Options{
			HostPort:  hostPort,
			Namespace: platformconfig.GetEnvOrDefaultString("TEMPORAL_NAMESPACE", "default"),
		})
	})
	return defaultTemporalClient, defaultTemporalClientErr
}
