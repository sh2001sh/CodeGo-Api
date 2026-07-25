package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type TaskListQuery struct {
	Platform       constant.TaskPlatform
	TaskID         string
	Status         string
	Action         string
	StartTimestamp int64
	EndTimestamp   int64
	ChannelID      string
}

func ListAllTasks(startIdx int, pageSize int, query TaskListQuery) ([]*dto.TaskDto, int64) {
	queryParams := workflowdomain.TaskListQuery{
		Platform:       query.Platform,
		TaskID:         query.TaskID,
		Status:         query.Status,
		Action:         query.Action,
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		ChannelID:      query.ChannelID,
	}

	items := workflowdomain.ListTasks(startIdx, pageSize, queryParams)
	total := workflowdomain.CountTasks(queryParams)
	return tasksToDTO(items, true), total
}

func ListUserTasks(userID int, startIdx int, pageSize int, query TaskListQuery) ([]*dto.TaskDto, int64) {
	queryParams := workflowdomain.TaskListQuery{
		Platform:       query.Platform,
		TaskID:         query.TaskID,
		Status:         query.Status,
		Action:         query.Action,
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
	}

	items := workflowdomain.ListUserTasks(userID, startIdx, pageSize, queryParams)
	total := workflowdomain.CountUserTasks(userID, queryParams)
	return tasksToDTO(items, false), total
}

func tasksToDTO(tasks []*workflowschema.Task, fillUser bool) []*dto.TaskDto {
	var userIDMap map[int]*identityschema.UserBase
	if fillUser {
		userIDs := types.NewSet[int]()
		for _, task := range tasks {
			userIDs.Add(task.UserId)
		}
		cacheUsers, err := listWorkflowUserBaseByIDs(userIDs.Items())
		if err == nil {
			userIDMap = cacheUsers
		} else {
			userIDMap = make(map[int]*identityschema.UserBase)
		}
	}

	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIDMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		result[i] = taskModelToDTO(task)
	}
	return result
}

func taskModelToDTO(task *workflowschema.Task) *dto.TaskDto {
	return &dto.TaskDto{
		ID:         task.ID,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		TaskID:     task.TaskID,
		Platform:   string(task.Platform),
		UserId:     task.UserId,
		Group:      task.Group,
		ChannelId:  task.ChannelId,
		Quota:      task.Quota,
		Action:     task.Action,
		Status:     string(task.Status),
		FailReason: task.FailReason,
		ResultURL:  task.GetResultURL(),
		SubmitTime: task.SubmitTime,
		StartTime:  task.StartTime,
		FinishTime: task.FinishTime,
		Progress:   task.Progress,
		Properties: task.Properties,
		Username:   task.Username,
		Data:       task.Data,
	}
}
