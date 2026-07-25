package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ProxyVideoContent validates ownership and proxies the completed task's video content.
func ProxyVideoContent(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		writeVideoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := workflowdomain.GetTaskByID(userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		writeVideoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		writeVideoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}
	if !workflowdomain.IsTaskSuccessStatus(task.Status) {
		writeVideoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}

	channel, err := gatewaystore.GetCachedChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		writeVideoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}

	videoURL, request, client, err := buildVideoProxyRequest(c.Request.Context(), channel, task)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to build proxy request for task %s: %s", taskID, err.Error()))
		writeVideoProxyError(c, http.StatusBadGateway, "server_error", err.Error())
		return
	}

	if strings.HasPrefix(videoURL, "data:") {
		if err := writeVideoDataURL(c, videoURL); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			writeVideoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	fetchSetting := platformstore.GetFetchSetting()
	if err := platformsecurity.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL blocked for task %s: %v", taskID, err))
		writeVideoProxyError(c, http.StatusForbidden, "server_error", fmt.Sprintf("request blocked: %v", err))
		return
	}

	request.URL, err = url.Parse(videoURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse URL %s: %s", videoURL, err.Error()))
		writeVideoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}

	resp, err := client.Do(request)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", videoURL, err.Error()))
		writeVideoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for %s", resp.StatusCode, videoURL))
		writeVideoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func writeVideoProxyError(c *gin.Context, status int, errType string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func buildVideoProxyRequest(ctx context.Context, channel *gatewayschema.Channel, task *workflowschema.Task) (string, *http.Request, *http.Client, error) {
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	proxy := gatewaydomain.GetSettings(channel).Proxy
	client, err := GetHttpClientWithProxy(proxy)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Failed to create proxy client")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "", nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Failed to create proxy request")
	}

	videoURL, err := resolveVideoURL(channel, task, baseURL, req)
	if err != nil {
		return "", nil, nil, err
	}
	if strings.TrimSpace(videoURL) == "" {
		return "", nil, nil, fmt.Errorf("Failed to fetch video content")
	}
	return strings.TrimSpace(videoURL), req, client, nil
}

func resolveVideoURL(channel *gatewayschema.Channel, task *workflowschema.Task, baseURL string, req *http.Request) (string, error) {
	switch channel.Type {
	case constant.ChannelTypeGemini:
		apiKey := task.PrivateData.Key
		if apiKey == "" {
			return "", fmt.Errorf("API key not stored for task")
		}
		videoURL, err := getGeminiVideoURL(channel, task, apiKey)
		if err != nil {
			return "", fmt.Errorf("Failed to resolve Gemini video URL")
		}
		req.Header.Set("x-goog-api-key", apiKey)
		return videoURL, nil
	case constant.ChannelTypeVertexAi:
		videoURL, err := getVertexVideoURL(channel, task)
		if err != nil {
			return "", fmt.Errorf("Failed to resolve Vertex video URL")
		}
		return videoURL, nil
	case constant.ChannelTypeOpenAI:
		req.Header.Set("Authorization", "Bearer "+channel.Key)
		return fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID()), nil
	default:
		return task.GetResultURL(), nil
	}
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return err
		}
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(videoBytes)
	return err
}

func getGeminiVideoURL(channel *gatewayschema.Channel, task *workflowschema.Task, apiKey string) (string, error) {
	if channel == nil || task == nil {
		return "", fmt.Errorf("invalid channel or task")
	}

	if uri := extractGeminiVideoURLFromTaskData(task); uri != "" {
		return ensureAPIKey(uri, apiKey), nil
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	if GetTaskRelayAdaptorFunc == nil {
		return "", fmt.Errorf("task adaptor factory not initialized")
	}
	adaptor := GetTaskRelayAdaptorFunc(constant.TaskPlatform(strconv.Itoa(channel.Type)))
	if adaptor == nil {
		return "", fmt.Errorf("gemini task adaptor not found")
	}
	if apiKey == "" {
		return "", fmt.Errorf("api key not available for task")
	}

	resp, err := adaptor.FetchTask(baseURL, apiKey, map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  task.Action,
	}, gatewaydomain.GetSettings(channel).Proxy)
	if err != nil {
		return "", fmt.Errorf("fetch task failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read task response failed: %w", err)
	}

	taskInfo, parseErr := adaptor.ParseTaskResult(body)
	if parseErr == nil && taskInfo != nil && taskInfo.RemoteUrl != "" {
		return ensureAPIKey(taskInfo.RemoteUrl, apiKey), nil
	}
	if uri := extractGeminiVideoURLFromPayload(body); uri != "" {
		return ensureAPIKey(uri, apiKey), nil
	}
	if parseErr != nil {
		return "", fmt.Errorf("parse task result failed: %w", parseErr)
	}
	return "", fmt.Errorf("gemini video url not found")
}

func extractGeminiVideoURLFromTaskData(task *workflowschema.Task) string {
	if task == nil || len(task.Data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := platformencoding.Unmarshal(task.Data, &payload); err != nil {
		return ""
	}
	return extractGeminiVideoURLFromMap(payload)
}

func extractGeminiVideoURLFromPayload(body []byte) string {
	var payload map[string]any
	if err := platformencoding.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return extractGeminiVideoURLFromMap(payload)
}

func extractGeminiVideoURLFromMap(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if uri, ok := payload["uri"].(string); ok && uri != "" {
		return uri
	}
	if resp, ok := payload["response"].(map[string]any); ok {
		if uri := extractGeminiVideoURLFromResponse(resp); uri != "" {
			return uri
		}
	}
	return ""
}

func extractGeminiVideoURLFromResponse(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	if gvr, ok := resp["generateVideoResponse"].(map[string]any); ok {
		if uri := extractGeminiVideoURLFromGeneratedSamples(gvr); uri != "" {
			return uri
		}
	}
	if videos, ok := resp["videos"].([]any); ok {
		for _, video := range videos {
			if vm, ok := video.(map[string]any); ok {
				if uri, ok := vm["uri"].(string); ok && uri != "" {
					return uri
				}
			}
		}
	}
	if uri, ok := resp["video"].(string); ok && uri != "" {
		return uri
	}
	if uri, ok := resp["uri"].(string); ok && uri != "" {
		return uri
	}
	return ""
}

func extractGeminiVideoURLFromGeneratedSamples(gvr map[string]any) string {
	if gvr == nil {
		return ""
	}
	if samples, ok := gvr["generatedSamples"].([]any); ok {
		for _, sample := range samples {
			if sm, ok := sample.(map[string]any); ok {
				if video, ok := sm["video"].(map[string]any); ok {
					if uri, ok := video["uri"].(string); ok && uri != "" {
						return uri
					}
				}
			}
		}
	}
	return ""
}

func getVertexVideoURL(channel *gatewayschema.Channel, task *workflowschema.Task) (string, error) {
	if channel == nil || task == nil {
		return "", fmt.Errorf("invalid channel or task")
	}
	if uri := strings.TrimSpace(task.GetResultURL()); uri != "" && !isTaskProxyContentURL(uri, task.TaskID) {
		return uri, nil
	}
	if uri := extractVertexVideoURLFromTaskData(task); uri != "" {
		return uri, nil
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	if GetTaskRelayAdaptorFunc == nil {
		return "", fmt.Errorf("task adaptor factory not initialized")
	}
	adaptor := GetTaskRelayAdaptorFunc(constant.TaskPlatform(strconv.Itoa(channel.Type)))
	if adaptor == nil {
		return "", fmt.Errorf("vertex task adaptor not found")
	}

	key := getVertexTaskKey(channel, task)
	if key == "" {
		return "", fmt.Errorf("vertex key not available for task")
	}

	resp, err := adaptor.FetchTask(baseURL, key, map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  task.Action,
	}, gatewaydomain.GetSettings(channel).Proxy)
	if err != nil {
		return "", fmt.Errorf("fetch task failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read task response failed: %w", err)
	}

	taskInfo, parseErr := adaptor.ParseTaskResult(body)
	if parseErr == nil && taskInfo != nil && strings.TrimSpace(taskInfo.Url) != "" {
		return taskInfo.Url, nil
	}
	if uri := extractVertexVideoURLFromPayload(body); uri != "" {
		return uri, nil
	}
	if parseErr != nil {
		return "", fmt.Errorf("parse task result failed: %w", parseErr)
	}
	return "", fmt.Errorf("vertex video url not found")
}

func isTaskProxyContentURL(uri string, taskID string) bool {
	if strings.TrimSpace(uri) == "" || strings.TrimSpace(taskID) == "" {
		return false
	}
	return strings.Contains(uri, "/v1/videos/"+taskID+"/content")
}

func getVertexTaskKey(channel *gatewayschema.Channel, task *workflowschema.Task) string {
	if task != nil {
		if key := strings.TrimSpace(task.PrivateData.Key); key != "" {
			return key
		}
	}
	if channel == nil {
		return ""
	}
	for _, key := range channel.GetKeys() {
		key = strings.TrimSpace(key)
		if key != "" {
			return key
		}
	}
	return strings.TrimSpace(channel.Key)
}

func extractVertexVideoURLFromTaskData(task *workflowschema.Task) string {
	if task == nil || len(task.Data) == 0 {
		return ""
	}
	return extractVertexVideoURLFromPayload(task.Data)
}

func extractVertexVideoURLFromPayload(body []byte) string {
	var payload map[string]any
	if err := platformencoding.Unmarshal(body, &payload); err != nil {
		return ""
	}
	resp, ok := payload["response"].(map[string]any)
	if !ok || resp == nil {
		return ""
	}

	if videos, ok := resp["videos"].([]any); ok && len(videos) > 0 {
		if video, ok := videos[0].(map[string]any); ok && video != nil {
			if b64, _ := video["bytesBase64Encoded"].(string); strings.TrimSpace(b64) != "" {
				mime, _ := video["mimeType"].(string)
				enc, _ := video["encoding"].(string)
				return buildVideoDataURL(mime, enc, b64)
			}
		}
	}
	if b64, _ := resp["bytesBase64Encoded"].(string); strings.TrimSpace(b64) != "" {
		enc, _ := resp["encoding"].(string)
		return buildVideoDataURL("", enc, b64)
	}
	if video, _ := resp["video"].(string); strings.TrimSpace(video) != "" {
		if strings.HasPrefix(video, "data:") || strings.HasPrefix(video, "http://") || strings.HasPrefix(video, "https://") {
			return video
		}
		enc, _ := resp["encoding"].(string)
		return buildVideoDataURL("", enc, video)
	}
	return ""
}

func buildVideoDataURL(mimeType string, encoding string, base64Data string) string {
	mime := strings.TrimSpace(mimeType)
	if mime == "" {
		enc := strings.TrimSpace(encoding)
		if enc == "" {
			enc = "mp4"
		}
		if strings.Contains(enc, "/") {
			mime = enc
		} else {
			mime = "video/" + enc
		}
	}
	return "data:" + mime + ";base64," + base64Data
}

func ensureAPIKey(uri string, key string) string {
	if key == "" || uri == "" || strings.Contains(uri, "key=") {
		return uri
	}
	if strings.Contains(uri, "?") {
		return fmt.Sprintf("%s&key=%s", uri, key)
	}
	return fmt.Sprintf("%s?key=%s", uri, key)
}
