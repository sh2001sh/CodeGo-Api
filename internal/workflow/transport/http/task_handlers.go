package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformpagination "github.com/sh2001sh/CodeGo-Api/internal/platform/pagination"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	workflowapp "github.com/sh2001sh/CodeGo-Api/internal/workflow/app"
	"strconv"
)

func GetAllTask(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	query := readTaskListQuery(c)

	items, total := workflowapp.ListAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), query)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetUserTask(c *gin.Context) {
	pageInfo := platformpagination.GetPageQuery(c)
	query := readTaskListQuery(c)

	items, total := workflowapp.ListUserTasks(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize(), query)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	httpapi.ApiSuccess(c, pageInfo)
}

func readTaskListQuery(c *gin.Context) workflowapp.TaskListQuery {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	return workflowapp.TaskListQuery{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
	}
}
