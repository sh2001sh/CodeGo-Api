package app

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

// TaskRelayAdaptor defines the adaptor contract required by task submit/fetch flows.
type TaskRelayAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError
	EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64
	AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64
	BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, err *dto.TaskError)
	FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
}

type OpenAIVideoTaskConverter interface {
	ConvertToOpenAIVideo(originTask *workflowschema.Task) ([]byte, error)
}

// GetTaskRelayAdaptorFunc is injected by bootstrap to avoid workflow -> relay package cycles.
var GetTaskRelayAdaptorFunc func(platform constant.TaskPlatform) TaskRelayAdaptor
