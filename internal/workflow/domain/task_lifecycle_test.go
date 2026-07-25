package domain

import (
	"encoding/json"
	"testing"

	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTimeoutLegacy(t *testing.T) {
	task := &workflowschema.Task{
		TaskID:     "legacy-timeout",
		Status:     workflowschema.TaskStatusInProgress,
		SubmitTime: 1740182399,
		Quota:      100,
	}

	decision := ApplyTimeout(task, 2000, 15, 1740182400)

	assert.Equal(t, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), task.Status)
	assert.Equal(t, TaskProgressComplete, task.Progress)
	assert.Equal(t, int64(2000), task.FinishTime)
	assert.Contains(t, decision.Reason, "旧系统遗留任务")
	assert.False(t, decision.ShouldRefund)
}

func TestApplyTimeoutRefundable(t *testing.T) {
	task := &workflowschema.Task{
		TaskID:     "normal-timeout",
		Status:     workflowschema.TaskStatusInProgress,
		SubmitTime: 1740182401,
		Quota:      100,
	}

	decision := ApplyTimeout(task, 3000, 20, 1740182400)

	assert.Equal(t, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), task.Status)
	assert.Equal(t, "任务超时（20分钟）", decision.Reason)
	assert.True(t, decision.ShouldRefund)
}

func TestShouldApplySunoTaskUpdate(t *testing.T) {
	task := &workflowschema.Task{
		Status:     workflowschema.TaskStatusInProgress,
		Progress:   "50%",
		SubmitTime: 1,
		StartTime:  2,
		FinishTime: 3,
		FailReason: "",
		Data:       json.RawMessage(`{"a":1}`),
	}
	same := dto.SunoDataResponse{
		Status:     string(workflowschema.TaskStatusInProgress),
		SubmitTime: 1,
		StartTime:  2,
		FinishTime: 3,
		Data:       json.RawMessage(`{"a":1}`),
	}
	changed := same
	changed.FailReason = "boom"

	assert.False(t, ShouldApplySunoTaskUpdate(task, same))
	assert.True(t, ShouldApplySunoTaskUpdate(task, changed))
}

func TestApplySunoTaskUpdateFailure(t *testing.T) {
	task := &workflowschema.Task{
		Status:   workflowschema.TaskStatusInProgress,
		Progress: "50%",
		Quota:    123,
	}
	update := dto.SunoDataResponse{
		Status:     string(workflowschema.TaskStatusFailure),
		FailReason: "failed",
		Data:       json.RawMessage(`{"done":true}`),
	}

	decision := ApplySunoTaskUpdate(task, update)

	assert.Equal(t, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), task.Status)
	assert.Equal(t, TaskProgressComplete, task.Progress)
	assert.Equal(t, "failed", task.FailReason)
	assert.True(t, decision.ShouldRefund)
	assert.Equal(t, "failed", decision.Reason)
	assert.JSONEq(t, `{"done":true}`, string(task.Data))
}

func TestApplyVideoTaskResultSuccess(t *testing.T) {
	task := &workflowschema.Task{
		TaskID:     "video-success",
		Status:     workflowschema.TaskStatusQueued,
		Quota:      100,
		Group:      "default",
		SubmitTime: 1,
	}
	result := &relaycommon.TaskInfo{
		Status:   string(workflowschema.TaskStatusSuccess),
		Url:      "https://example.com/video.mp4",
		Progress: "100%",
	}

	decision, err := ApplyVideoTaskResult(task, result, 5000)
	require.NoError(t, err)

	assert.Equal(t, workflowschema.TaskStatus(workflowschema.TaskStatusSuccess), task.Status)
	assert.Equal(t, TaskProgressComplete, task.Progress)
	assert.Equal(t, int64(5000), task.FinishTime)
	assert.Equal(t, "https://example.com/video.mp4", task.PrivateData.ResultURL)
	assert.True(t, decision.ShouldSettle)
	assert.False(t, decision.ShouldRefund)
}

func TestApplyVideoTaskResultFailure(t *testing.T) {
	task := &workflowschema.Task{
		TaskID: "video-fail",
		Status: workflowschema.TaskStatusInProgress,
		Quota:  100,
	}
	result := &relaycommon.TaskInfo{
		Status: string(workflowschema.TaskStatusFailure),
		Reason: "upstream failed",
	}

	decision, err := ApplyVideoTaskResult(task, result, 6000)
	require.NoError(t, err)

	assert.Equal(t, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), task.Status)
	assert.Equal(t, TaskProgressComplete, task.Progress)
	assert.Equal(t, int64(6000), task.FinishTime)
	assert.Equal(t, "upstream failed", task.FailReason)
	assert.True(t, decision.ShouldRefund)
	assert.False(t, decision.ShouldSettle)
	assert.Equal(t, TaskProgressComplete, result.Progress)
}

func TestApplyVideoTaskResultUnknownStatus(t *testing.T) {
	task := &workflowschema.Task{TaskID: "video-unknown"}
	result := &relaycommon.TaskInfo{Status: "BROKEN"}

	decision, err := ApplyVideoTaskResult(task, result, 1)

	require.Error(t, err)
	assert.False(t, decision.ShouldRefund)
	assert.False(t, decision.ShouldSettle)
}
