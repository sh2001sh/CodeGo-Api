package domain

import workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"

type TaskStatus = workflowschema.TaskStatus

const (
	TaskStatusNotStart   = workflowschema.TaskStatusNotStart
	TaskStatusSubmitted  = workflowschema.TaskStatusSubmitted
	TaskStatusQueued     = workflowschema.TaskStatusQueued
	TaskStatusInProgress = workflowschema.TaskStatusInProgress
	TaskStatusFailure    = workflowschema.TaskStatusFailure
	TaskStatusSuccess    = workflowschema.TaskStatusSuccess
	TaskStatusUnknown    = workflowschema.TaskStatusUnknown
)

func ParseTaskStatus(status string) TaskStatus {
	return TaskStatus(status)
}

func IsTaskSuccessStatus(status TaskStatus) bool {
	return status == TaskStatusSuccess
}

func IsTaskFailureStatus(status TaskStatus) bool {
	return status == TaskStatusFailure
}

func IsTaskTerminalStatus(status TaskStatus) bool {
	return IsTaskSuccessStatus(status) || IsTaskFailureStatus(status)
}

func TaskStatusToSimple(status TaskStatus) string {
	switch status {
	case TaskStatusSuccess:
		return "succeeded"
	case TaskStatusFailure:
		return "failed"
	case TaskStatusQueued, TaskStatusSubmitted:
		return "queued"
	default:
		return "processing"
	}
}
