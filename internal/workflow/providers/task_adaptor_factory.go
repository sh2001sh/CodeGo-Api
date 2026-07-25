package providers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	taskali "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/ali"
	taskdoubao "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/doubao"
	taskgemini "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/gemini"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/hailuo"
	taskjimeng "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/jimeng"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/kling"
	taskopenaivideo "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/openai_video"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/suno"
	taskvertex "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/vertex"
	taskvidu "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/vidu"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

type TaskRuntimeAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError
	EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64
	AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64
	BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, err *dto.TaskError)
	FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
	AdjustBillingOnComplete(task *workflowschema.Task, taskResult *relaycommon.TaskInfo) int
}

func NewTaskRuntimeAdaptor(platform constant.TaskPlatform) TaskRuntimeAdaptor {
	switch platform {
	case constant.TaskPlatformSuno:
		return &suno.TaskAdaptor{}
	}

	channelType, err := strconv.ParseInt(string(platform), 10, 64)
	if err != nil {
		return nil
	}

	switch channelType {
	case constant.ChannelTypeAli:
		return &taskali.TaskAdaptor{}
	case constant.ChannelTypeKling:
		return &kling.TaskAdaptor{}
	case constant.ChannelTypeJimeng:
		return &taskjimeng.TaskAdaptor{}
	case constant.ChannelTypeVertexAi:
		return &taskvertex.TaskAdaptor{}
	case constant.ChannelTypeVidu:
		return &taskvidu.TaskAdaptor{}
	case constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine:
		return &taskdoubao.TaskAdaptor{}
	case constant.ChannelTypeOpenAI:
		return &taskopenaivideo.TaskAdaptor{}
	case constant.ChannelTypeGemini:
		return &taskgemini.TaskAdaptor{}
	case constant.ChannelTypeMiniMax:
		return &hailuo.TaskAdaptor{}
	default:
		return nil
	}
}
