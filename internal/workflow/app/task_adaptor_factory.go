package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	workflowproviders "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers"
)

type taskRuntimeAdaptor interface {
	TaskPollingAdaptor
	TaskRelayAdaptor
}

func newTaskRuntimeAdaptor(platform constant.TaskPlatform) taskRuntimeAdaptor {
	return workflowproviders.NewTaskRuntimeAdaptor(platform)
}

func NewTaskPollingAdaptor(platform constant.TaskPlatform) TaskPollingAdaptor {
	return newTaskRuntimeAdaptor(platform)
}

func NewTaskRelayAdaptor(platform constant.TaskPlatform) TaskRelayAdaptor {
	return newTaskRuntimeAdaptor(platform)
}
