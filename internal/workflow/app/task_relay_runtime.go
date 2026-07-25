package app

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/taskx"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowproviders "github.com/sh2001sh/CodeGo-Api/internal/workflow/providers"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type TaskSubmitResult struct {
	UpstreamTaskID string
	TaskData       []byte
	Platform       constant.TaskPlatform
	Quota          int
}

func taskPlatformFromContext(c *gin.Context) constant.TaskPlatform {
	channelType := c.GetInt("channel_type")
	if channelType > 0 {
		return constant.TaskPlatform(strconv.Itoa(channelType))
	}
	return constant.TaskPlatform(c.GetString("platform"))
}

// ResolveOriginTask handles task submission derived from an existing task, such as remix.
func ResolveOriginTask(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	path := c.Request.URL.Path
	if strings.Contains(path, "/v1/videos/") && strings.HasSuffix(path, "/remix") {
		info.Action = constant.TaskActionRemix
	}

	if info.Action == constant.TaskActionRemix {
		videoID := c.Param("video_id")
		if strings.TrimSpace(videoID) == "" {
			return taskx.TaskErrorWrapperLocal(fmt.Errorf("video_id is required"), "invalid_request", http.StatusBadRequest)
		}
		info.OriginTaskID = videoID
	}

	if info.OriginTaskID == "" {
		return nil
	}

	originTask, exist, err := workflowdomain.GetTaskByID(info.UserId, info.OriginTaskID)
	if err != nil {
		return taskx.TaskErrorWrapper(err, "get_origin_task_failed", http.StatusInternalServerError)
	}
	if !exist {
		return taskx.TaskErrorWrapperLocal(errors.New("task_origin_not_exist"), "task_not_exist", http.StatusBadRequest)
	}

	if info.OriginModelName == "" {
		if originTask.Properties.OriginModelName != "" {
			info.OriginModelName = originTask.Properties.OriginModelName
		} else if originTask.Properties.UpstreamModelName != "" {
			info.OriginModelName = originTask.Properties.UpstreamModelName
		} else {
			var taskData map[string]interface{}
			_ = platformencoding.Unmarshal(originTask.Data, &taskData)
			if m, ok := taskData["model"].(string); ok && m != "" {
				info.OriginModelName = m
			}
		}
	}

	ch, err := gatewaystore.LoadChannelByID(originTask.ChannelId, true)
	if err != nil {
		return taskx.TaskErrorWrapperLocal(err, "channel_not_found", http.StatusBadRequest)
	}
	if ch.Status != constant.ChannelStatusEnabled {
		return taskx.TaskErrorWrapperLocal(errors.New("the channel of the origin task is disabled"), "task_channel_disable", http.StatusBadRequest)
	}
	info.LockedChannel = ch

	if originTask.ChannelId != info.ChannelId {
		key, _, apiError := gatewaystore.GetNextEnabledChannelKey(ch)
		if apiError != nil {
			return taskx.TaskErrorWrapper(apiError, "channel_no_available_key", apiError.StatusCode)
		}
		httpctx.SetContextKey(c, constant.ContextKeyChannelKey, key)
		httpctx.SetContextKey(c, constant.ContextKeyChannelType, ch.Type)
		httpctx.SetContextKey(c, constant.ContextKeyChannelBaseUrl, ch.GetBaseURL())
		httpctx.SetContextKey(c, constant.ContextKeyChannelId, originTask.ChannelId)

		info.ChannelBaseUrl = ch.GetBaseURL()
		info.ChannelId = originTask.ChannelId
		info.ChannelType = ch.Type
		info.ApiKey = key
	}

	if info.Action == constant.TaskActionRemix {
		if originTask.PrivateData.BillingContext != nil {
			for key, value := range originTask.PrivateData.BillingContext.OtherRatios {
				info.PriceData.AddOtherRatio(key, value)
			}
		} else {
			var taskData map[string]interface{}
			_ = platformencoding.Unmarshal(originTask.Data, &taskData)
			secondsStr, _ := taskData["seconds"].(string)
			seconds, _ := strconv.Atoi(secondsStr)
			if seconds <= 0 {
				seconds = 4
			}
			sizeStr, _ := taskData["size"].(string)
			if info.PriceData.OtherRatios == nil {
				info.PriceData.OtherRatios = map[string]float64{}
			}
			info.PriceData.OtherRatios["seconds"] = float64(seconds)
			info.PriceData.OtherRatios["size"] = 1
			if sizeStr == "1792x1024" || sizeStr == "1024x1792" {
				info.PriceData.OtherRatios["size"] = 1.666667
			}
		}
	}

	return nil
}

// RelayTaskSubmit completes one async task submission attempt.
func RelayTaskSubmit(c *gin.Context, info *relaycommon.RelayInfo) (*TaskSubmitResult, *dto.TaskError) {
	info.InitChannelMeta(c)

	platform := constant.TaskPlatform(c.GetString("platform"))
	if platform == "" {
		platform = taskPlatformFromContext(c)
	}

	if GetTaskRelayAdaptorFunc == nil {
		return nil, taskx.TaskErrorWrapperLocal(errors.New("task adaptor factory not initialized"), "task_adaptor_factory_not_initialized", http.StatusInternalServerError)
	}
	adaptor := GetTaskRelayAdaptorFunc(platform)
	if adaptor == nil {
		return nil, taskx.TaskErrorWrapperLocal(fmt.Errorf("invalid api platform: %s", platform), "invalid_api_platform", http.StatusBadRequest)
	}

	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		return nil, taskErr
	}

	modelName := info.OriginModelName
	if modelName == "" {
		modelName = taskx.CoverTaskActionToModelName(platform, info.Action)
	}

	info.OriginModelName = modelName
	info.UpstreamModelName = modelName
	if err := relaycommon.ModelMappedHelper(c, info, nil); err != nil {
		return nil, taskx.TaskErrorWrapperLocal(err, "model_mapping_failed", http.StatusBadRequest)
	}

	if info.PublicTaskID == "" {
		info.PublicTaskID = workflowdomain.GeneratePublicTaskID()
	}

	priceData, err := relaycommon.ModelPriceHelperPerCall(c, info)
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "model_price_error", http.StatusBadRequest)
	}
	info.PriceData = priceData

	if estimatedRatios := adaptor.EstimateBilling(c, info); len(estimatedRatios) > 0 {
		for key, value := range estimatedRatios {
			info.PriceData.AddOtherRatio(key, value)
		}
	}

	if !platformtext.StringsContains(constant.TaskPricePatches, modelName) {
		for _, ratio := range info.PriceData.OtherRatios {
			if ratio != 1.0 {
				info.PriceData.Quota = int(float64(info.PriceData.Quota) * ratio)
			}
		}
	}

	if info.Billing == nil && !info.PriceData.FreeModel {
		info.ForcePreConsume = true
		if apiErr := billingapp.PreConsumeRelayBilling(c, info.PriceData.Quota, info); apiErr != nil {
			return nil, taskx.TaskErrorFromAPIError(apiErr)
		}
	}

	requestBody, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "build_request_failed", http.StatusInternalServerError)
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	if resp != nil && resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, taskx.TaskErrorWrapper(fmt.Errorf("%s", string(responseBody)), "fail_to_fetch_task", resp.StatusCode)
	}

	otherRatios := info.PriceData.OtherRatios
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	ratiosJSON, _ := platformencoding.Marshal(otherRatios)
	c.Header("X-CodeGo-Api-Other-Ratios", string(ratiosJSON))

	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(c, resp, info)
	if taskErr != nil {
		return nil, taskErr
	}

	finalQuota := info.PriceData.Quota
	if adjustedRatios := adaptor.AdjustBillingOnSubmit(info, taskData); len(adjustedRatios) > 0 {
		finalQuota = recalcQuotaFromRatios(info, adjustedRatios)
		info.PriceData.OtherRatios = adjustedRatios
		info.PriceData.Quota = finalQuota
	}

	return &TaskSubmitResult{
		UpstreamTaskID: upstreamTaskID,
		TaskData:       taskData,
		Platform:       platform,
		Quota:          finalQuota,
	}, nil
}

func recalcQuotaFromRatios(info *relaycommon.RelayInfo, ratios map[string]float64) int {
	baseQuota := info.PriceData.Quota
	for _, ratio := range info.PriceData.OtherRatios {
		if ratio != 1.0 && ratio > 0 {
			baseQuota = int(float64(baseQuota) / ratio)
		}
	}

	result := float64(baseQuota)
	for _, ratio := range ratios {
		if ratio != 1.0 {
			result *= ratio
		}
	}
	return int(result)
}

var fetchRespBuilders = map[int]func(c *gin.Context) (respBody []byte, taskResp *dto.TaskError){
	gatewaycontract.RelayModeSunoFetchByID:  sunoFetchByIDRespBodyBuilder,
	gatewaycontract.RelayModeSunoFetch:      sunoFetchRespBodyBuilder,
	gatewaycontract.RelayModeVideoFetchByID: videoFetchByIDRespBodyBuilder,
}

// RelayTaskFetch handles public async task fetch requests.
func RelayTaskFetch(c *gin.Context, relayMode int) (taskResp *dto.TaskError) {
	respBuilder, ok := fetchRespBuilders[relayMode]
	if !ok {
		return taskx.TaskErrorWrapperLocal(errors.New("invalid_relay_mode"), "invalid_relay_mode", http.StatusBadRequest)
	}

	respBody, taskErr := respBuilder(c)
	if taskErr != nil {
		return taskErr
	}
	if len(respBody) == 0 {
		respBody = []byte("{\"code\":\"success\",\"data\":null}")
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(c.Writer, bytes.NewBuffer(respBody)); err != nil {
		return taskx.TaskErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return nil
}

func sunoFetchRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	userID := c.GetInt("id")
	var condition struct {
		IDs    []any  `json:"ids"`
		Action string `json:"action"`
	}
	if err := c.BindJSON(&condition); err != nil {
		return nil, taskx.TaskErrorWrapper(err, "invalid_request", http.StatusBadRequest)
	}

	tasks := make([]any, 0, len(condition.IDs))
	if len(condition.IDs) > 0 {
		taskModels, err := workflowdomain.GetTasksByIDs(userID, condition.IDs)
		if err != nil {
			return nil, taskx.TaskErrorWrapper(err, "get_tasks_failed", http.StatusInternalServerError)
		}
		for _, task := range taskModels {
			tasks = append(tasks, taskModelToDTO(task))
		}
	}

	respBody, err := platformencoding.Marshal(dto.TaskResponse[[]any]{
		Code: "success",
		Data: tasks,
	})
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return respBody, nil
}

func sunoFetchByIDRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	taskID := c.Param("id")
	userID := c.GetInt("id")

	originTask, exist, err := workflowdomain.GetTaskByID(userID, taskID)
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError)
	}
	if !exist {
		return nil, taskx.TaskErrorWrapperLocal(errors.New("task_not_exist"), "task_not_exist", http.StatusBadRequest)
	}

	respBody, err = platformencoding.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: taskModelToDTO(originTask),
	})
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return respBody, nil
}

func videoFetchByIDRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	taskID := c.Param("task_id")
	if taskID == "" {
		taskID = c.GetString("task_id")
	}
	userID := c.GetInt("id")

	originTask, exist, err := workflowdomain.GetTaskByID(userID, taskID)
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError)
	}
	if !exist {
		return nil, taskx.TaskErrorWrapperLocal(errors.New("task_not_exist"), "task_not_exist", http.StatusBadRequest)
	}

	isOpenAIVideoAPI := strings.HasPrefix(c.Request.RequestURI, "/v1/videos/")
	if realtimeResp := tryRealtimeFetch(originTask, isOpenAIVideoAPI); len(realtimeResp) > 0 {
		return realtimeResp, nil
	}

	if isOpenAIVideoAPI {
		if GetTaskRelayAdaptorFunc == nil {
			return nil, taskx.TaskErrorWrapperLocal(errors.New("task adaptor factory not initialized"), "task_adaptor_factory_not_initialized", http.StatusInternalServerError)
		}
		adaptor := GetTaskRelayAdaptorFunc(originTask.Platform)
		if adaptor == nil {
			return nil, taskx.TaskErrorWrapperLocal(fmt.Errorf("invalid channel id: %d", originTask.ChannelId), "invalid_channel_id", http.StatusBadRequest)
		}
		if converter, ok := adaptor.(OpenAIVideoTaskConverter); ok {
			openAIVideoData, err := converter.ConvertToOpenAIVideo(originTask)
			if err != nil {
				return nil, taskx.TaskErrorWrapper(err, "convert_to_openai_video_failed", http.StatusInternalServerError)
			}
			return openAIVideoData, nil
		}
		return nil, taskx.TaskErrorWrapperLocal(fmt.Errorf("not_implemented:%s", originTask.Platform), "not_implemented", http.StatusNotImplemented)
	}

	respBody, err = platformencoding.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: taskModelToDTO(originTask),
	})
	if err != nil {
		return nil, taskx.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return respBody, nil
}

func tryRealtimeFetch(task *workflowschema.Task, isOpenAIVideoAPI bool) []byte {
	channelModel, err := gatewaystore.LoadChannelByID(task.ChannelId, true)
	if err != nil {
		return nil
	}
	if channelModel.Type != constant.ChannelTypeVertexAi && channelModel.Type != constant.ChannelTypeGemini {
		return nil
	}
	if GetTaskRelayAdaptorFunc == nil {
		return nil
	}

	baseURL := constant.ChannelBaseURLs[channelModel.Type]
	if channelModel.GetBaseURL() != "" {
		baseURL = channelModel.GetBaseURL()
	}
	proxy := gatewaydomain.GetSettings(channelModel).Proxy
	adaptor := GetTaskRelayAdaptorFunc(constant.TaskPlatform(strconv.Itoa(channelModel.Type)))
	if adaptor == nil {
		return nil
	}

	resp, err := adaptor.FetchTask(baseURL, channelModel.Key, map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  task.Action,
	}, proxy)
	if err != nil || resp == nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	taskInfo, err := adaptor.ParseTaskResult(body)
	if err != nil || taskInfo == nil {
		return nil
	}

	snap := workflowdomain.TakeTaskSnapshot(task)

	if taskInfo.Status != "" {
		task.Status = workflowdomain.ParseTaskStatus(taskInfo.Status)
	}
	if taskInfo.Progress != "" {
		task.Progress = taskInfo.Progress
	}
	if strings.HasPrefix(taskInfo.Url, "data:") {
	} else if taskInfo.Url != "" {
		task.PrivateData.ResultURL = taskInfo.Url
	} else if workflowdomain.IsTaskSuccessStatus(task.Status) {
		task.PrivateData.ResultURL = workflowproviders.BuildTaskProxyURL(task.TaskID)
	}

	if !snap.Equal(workflowdomain.TakeTaskSnapshot(task)) {
		_, _ = workflowdomain.UpdateTaskWithStatus(task, snap.Status)
	}

	if isOpenAIVideoAPI {
		return nil
	}

	format := detectVideoFormat(body)
	respBody, _ := platformencoding.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: map[string]any{
			"error":    nil,
			"format":   format,
			"metadata": nil,
			"status":   workflowdomain.TaskStatusToSimple(task.Status),
			"task_id":  task.TaskID,
			"url":      task.GetResultURL(),
		},
	})
	return respBody
}

func detectVideoFormat(rawBody []byte) string {
	var raw map[string]any
	if err := platformencoding.Unmarshal(rawBody, &raw); err != nil {
		return "mp4"
	}
	respObj, ok := raw["response"].(map[string]any)
	if !ok {
		return "mp4"
	}
	vids, ok := respObj["videos"].([]any)
	if !ok || len(vids) == 0 {
		return "mp4"
	}
	video, ok := vids[0].(map[string]any)
	if !ok {
		return "mp4"
	}
	mimeType, ok := video["mimeType"].(string)
	if !ok || mimeType == "" || strings.Contains(mimeType, "mp4") {
		return "mp4"
	}
	return mimeType
}
