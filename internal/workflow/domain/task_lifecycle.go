package domain

import (
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"sort"
	"strings"
)

const (
	TaskProgressSubmitted  = "10%"
	TaskProgressQueued     = "20%"
	TaskProgressInProgress = "30%"
	TaskProgressComplete   = "100%"
)

type TimeoutDecision struct {
	Reason       string
	ShouldRefund bool
}

type VideoTransitionDecision struct {
	ShouldRefund bool
	ShouldSettle bool
}

func ApplyTimeout(task *workflowschema.Task, now int64, timeoutMinutes int, legacyTaskCutoff int64) TimeoutDecision {
	if task == nil {
		return TimeoutDecision{}
	}
	reason := fmt.Sprintf("任务超时（%d分钟）", timeoutMinutes)
	legacyReason := "任务超时（旧系统遗留任务，不进行退款，请联系管理员）"
	isLegacy := task.SubmitTime > 0 && task.SubmitTime < legacyTaskCutoff

	task.Status = TaskStatusFailure
	task.Progress = TaskProgressComplete
	task.FinishTime = now
	if isLegacy {
		task.FailReason = legacyReason
		return TimeoutDecision{Reason: legacyReason}
	}
	task.FailReason = reason
	return TimeoutDecision{
		Reason:       reason,
		ShouldRefund: task.Quota != 0,
	}
}

func ShouldApplySunoTaskUpdate(oldTask *workflowschema.Task, newTask dto.SunoDataResponse) bool {
	if oldTask == nil {
		return false
	}
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if string(oldTask.Status) != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}
	if IsTaskTerminalStatus(oldTask.Status) && oldTask.Progress != TaskProgressComplete {
		return true
	}

	oldData, _ := platformencoding.Marshal(oldTask.Data)
	newData, _ := platformencoding.Marshal(newTask.Data)
	sort.Slice(oldData, func(i, j int) bool { return oldData[i] < oldData[j] })
	sort.Slice(newData, func(i, j int) bool { return newData[i] < newData[j] })
	return string(oldData) != string(newData)
}

func ApplySunoTaskUpdate(task *workflowschema.Task, response dto.SunoDataResponse) TimeoutDecision {
	if task == nil {
		return TimeoutDecision{}
	}
	task.Status = ParseTaskStatus(response.Status)
	if response.FailReason != "" {
		task.FailReason = response.FailReason
	}
	if response.SubmitTime != 0 {
		task.SubmitTime = response.SubmitTime
	}
	if response.StartTime != 0 {
		task.StartTime = response.StartTime
	}
	if response.FinishTime != 0 {
		task.FinishTime = response.FinishTime
	}
	if response.FailReason != "" || IsTaskFailureStatus(task.Status) {
		task.Progress = TaskProgressComplete
		task.Data = response.Data
		return TimeoutDecision{
			Reason:       task.FailReason,
			ShouldRefund: task.Quota != 0,
		}
	}
	if response.Status == string(TaskStatusSuccess) {
		task.Progress = TaskProgressComplete
	}
	task.Data = response.Data
	return TimeoutDecision{}
}

func ApplyVideoTaskResult(task *workflowschema.Task, taskResult *relaycommon.TaskInfo, now int64) (VideoTransitionDecision, error) {
	if task == nil || taskResult == nil {
		return VideoTransitionDecision{}, nil
	}

	task.Status = ParseTaskStatus(taskResult.Status)
	switch task.Status {
	case TaskStatusSubmitted:
		task.Progress = TaskProgressSubmitted
	case TaskStatusQueued:
		task.Progress = TaskProgressQueued
	case TaskStatusInProgress:
		task.Progress = TaskProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case TaskStatusSuccess:
		task.Progress = TaskProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if strings.HasPrefix(taskResult.Url, "data:") {
			task.PrivateData.ResultURL = BuildTaskProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			task.PrivateData.ResultURL = taskResult.Url
		} else {
			task.PrivateData.ResultURL = BuildTaskProxyURL(task.TaskID)
		}
		return VideoTransitionDecision{ShouldSettle: true}, nil
	case TaskStatusFailure:
		task.Status = TaskStatusFailure
		task.Progress = TaskProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		taskResult.Progress = TaskProgressComplete
		return VideoTransitionDecision{
			ShouldRefund: task.Quota != 0,
		}, nil
	default:
		return VideoTransitionDecision{}, fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}

	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}
	return VideoTransitionDecision{}, nil
}

func BuildTaskProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/videos/%s/content", platformconfig.ServerAddress, taskID)
}
