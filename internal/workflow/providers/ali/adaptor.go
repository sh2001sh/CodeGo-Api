package ali

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/taskx"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/taskcommon"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/providers/taskhttp"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

type AliVideoInput struct {
	Prompt         string `json:"prompt,omitempty"`
	ImgURL         string `json:"img_url,omitempty"`
	FirstFrameURL  string `json:"first_frame_url,omitempty"`
	LastFrameURL   string `json:"last_frame_url,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Template       string `json:"template,omitempty"`
}

type AliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`
	Size         string `json:"size,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	PromptExtend bool   `json:"prompt_extend,omitempty"`
	Watermark    bool   `json:"watermark,omitempty"`
	Audio        *bool  `json:"audio,omitempty"`
	Seed         int    `json:"seed,omitempty"`
}

type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

type AliUsage struct {
	Duration   dto.IntValue `json:"duration,omitempty"`
	VideoCount dto.IntValue `json:"video_count,omitempty"`
	SR         dto.IntValue `json:"SR,omitempty"`
}

type AliMetadata struct {
	AudioURL       string  `json:"audio_url,omitempty"`
	ImgURL         string  `json:"img_url,omitempty"`
	FirstFrameURL  string  `json:"first_frame_url,omitempty"`
	LastFrameURL   string  `json:"last_frame_url,omitempty"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	Template       string  `json:"template,omitempty"`
	Resolution     *string `json:"resolution,omitempty"`
	Size           *string `json:"size,omitempty"`
	Duration       *int    `json:"duration,omitempty"`
	PromptExtend   *bool   `json:"prompt_extend,omitempty"`
	Watermark      *bool   `json:"watermark,omitempty"`
	Audio          *bool   `json:"audio,omitempty"`
	Seed           *int    `json:"seed,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v1/services/aigc/video-generation/video-synthesis", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := platformencoding.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p  = []string{"832*480", "480*832", "624*624"}
	size720p  = []string{"1280*720", "720*1280", "960*960", "1088*832", "832*1088"}
	size1080p = []string{"1920*1080", "1080*1920", "1440*1440", "1632*1248", "1248*1632"}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	} else if lo.Contains(size720p, size) {
		return "720P", nil
	} else if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func ProcessAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	otherRatios := make(map[string]float64)
	aliRatios := map[string]map[string]float64{
		"wan2.6-i2v":         {"720P": 1, "1080P": 1 / 0.6},
		"wan2.5-t2v-preview": {"480P": 1, "720P": 2, "1080P": 1 / 0.3},
		"wan2.2-t2v-plus":    {"480P": 1, "1080P": 0.7 / 0.14},
		"wan2.5-i2v-preview": {"480P": 1, "720P": 2, "1080P": 1 / 0.3},
		"wan2.2-i2v-plus":    {"480P": 1, "1080P": 0.7 / 0.14},
		"wan2.2-kf2v-flash":  {"480P": 1, "720P": 2, "1080P": 4.8},
		"wan2.2-i2v-flash":   {"480P": 1, "720P": 2},
		"wan2.2-s2v":         {"480P": 1, "720P": 0.9 / 0.5},
	}
	var resolution string

	if aliReq.Parameters.Size != "" {
		toResolution, err := sizeToResolution(aliReq.Parameters.Size)
		if err != nil {
			return nil, err
		}
		resolution = toResolution
	} else {
		resolution = strings.ToUpper(aliReq.Parameters.Resolution)
		if !strings.HasSuffix(resolution, "P") {
			resolution = resolution + "P"
		}
	}
	if otherRatio, ok := aliRatios[aliReq.Model]; ok {
		if ratio, ok := otherRatio[resolution]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", resolution)] = ratio
		}
	}
	return otherRatios, nil
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
			ImgURL: req.InputReference,
		},
		Parameters: &AliVideoParameters{
			PromptExtend: true,
			Watermark:    false,
		},
	}

	if req.Size != "" {
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			return nil, fmt.Errorf("invalid size: %s, example: %s", req.Size, "1920*1080")
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else {
			resolution := strings.ToUpper(req.Size)
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	} else {
		if strings.Contains(req.Model, "t2v") {
			if strings.HasPrefix(req.Model, "wan2.5") || strings.HasPrefix(req.Model, "wan2.2") {
				aliReq.Parameters.Size = "1920*1080"
			} else {
				aliReq.Parameters.Size = "1280*720"
			}
		} else {
			if strings.HasPrefix(req.Model, "wan2.6") || strings.HasPrefix(req.Model, "wan2.5") || strings.HasPrefix(req.Model, "wan2.2-i2v-plus") {
				aliReq.Parameters.Resolution = "1080P"
			} else {
				aliReq.Parameters.Resolution = "720P"
			}
		}
	}

	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			return nil, errors.Wrap(err, "convert seconds to int failed")
		}
		aliReq.Parameters.Duration = seconds
	}

	if req.Metadata != nil {
		if metadataBytes, err := platformencoding.Marshal(req.Metadata); err == nil {
			err = platformencoding.Unmarshal(metadataBytes, aliReq)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Parameters.Duration <= 0 {
		aliReq.Parameters.Duration = 5
	}
	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}
	return aliReq, nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil
	}

	otherRatios := map[string]float64{
		"seconds": float64(aliReq.Parameters.Duration),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	return otherRatios
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return taskhttp.DoTaskAPIRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = taskx.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var aliResp AliVideoResponse
	if err := platformencoding.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = taskx.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	if aliResp.Code != "" {
		taskErr = taskx.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}
	if aliResp.Output.TaskID == "" {
		taskErr = taskx.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = platformruntime.GetTimestamp()
	c.JSON(http.StatusOK, openAIResp)

	return aliResp.Output.TaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseURL, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := platformhttpx.GetHTTPClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }
func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := platformencoding.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{Code: 0}
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = workflowschema.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = workflowschema.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = workflowschema.TaskStatusSuccess
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = workflowschema.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = workflowschema.TaskStatusQueued
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *workflowschema.Task) ([]byte, error) {
	var aliResp AliVideoResponse
	if err := platformencoding.Unmarshal(task.Data, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal ali response failed")
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt
	openAIResp.SetMetadata("url", aliResp.Output.VideoURL)

	if aliResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: aliResp.Code, Message: aliResp.Message}
	} else if aliResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: aliResp.Output.Code, Message: aliResp.Output.Message}
	}
	return platformencoding.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}
