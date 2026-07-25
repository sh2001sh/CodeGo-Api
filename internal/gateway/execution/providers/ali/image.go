package ali

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

func oaiImage2AliImageRequest(info *relaycommon.RelayInfo, request dto.ImageRequest, isSync bool) (*AliImageRequest, error) {
	var imageRequest AliImageRequest
	imageRequest.Model = request.Model
	imageRequest.ResponseFormat = request.ResponseFormat
	if request.Extra != nil {
		if val, ok := request.Extra["parameters"]; ok {
			if err := platformencoding.Unmarshal(val, &imageRequest.Parameters); err != nil {
				return nil, fmt.Errorf("invalid parameters field: %w", err)
			}
		} else {
			imageRequest.Parameters = AliImageParameters{
				Size:      strings.Replace(request.Size, "x", "*", -1),
				N:         int(lo.FromPtrOr(request.N, uint(1))),
				Watermark: request.Watermark,
			}
		}
		if val, ok := request.Extra["input"]; ok {
			if err := platformencoding.Unmarshal(val, &imageRequest.Input); err != nil {
				return nil, fmt.Errorf("invalid input field: %w", err)
			}
		}
	}

	if strings.Contains(request.Model, "z-image") && imageRequest.Parameters.PromptExtendValue() {
		info.PriceData.AddOtherRatio("prompt_extend", 2)
	}
	if imageRequest.Parameters.N != 0 {
		info.PriceData.AddOtherRatio("n", float64(imageRequest.Parameters.N))
	}

	if isSync {
		if imageRequest.Input == nil {
			imageRequest.Input = AliImageInput{
				Messages: []AliMessage{{
					Role: "user",
					Content: []AliMediaContent{{
						Text: request.Prompt,
					}},
				}},
			}
		}
	} else if imageRequest.Input == nil {
		imageRequest.Input = AliImageInput{Prompt: request.Prompt}
	}

	return &imageRequest, nil
}

func getImageBase64sFromForm(c *gin.Context, fieldName string) ([]string, error) {
	mf := c.Request.MultipartForm
	if mf == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
		}
		mf = c.Request.MultipartForm
	}

	var (
		imageFiles []*multipart.FileHeader
		exists     bool
	)

	if imageFiles, exists = mf.File[fieldName]; !exists || len(imageFiles) == 0 {
		if imageFiles, exists = mf.File[fieldName+"[]"]; !exists || len(imageFiles) == 0 {
			foundArrayImages := false
			for formFieldName, files := range mf.File {
				if strings.HasPrefix(formFieldName, fieldName+"[") && len(files) > 0 {
					foundArrayImages = true
					imageFiles = append(imageFiles, files...)
				}
			}
			if !foundArrayImages && len(imageFiles) == 0 {
				return nil, errors.New("image is required")
			}
		}
	}

	if len(imageFiles) == 0 {
		return nil, errors.New("image is required")
	}

	var imageBase64s []string
	for _, file := range imageFiles {
		image, err := file.Open()
		if err != nil {
			return nil, errors.New("failed to open image file")
		}

		imageData, err := io.ReadAll(image)
		_ = image.Close()
		if err != nil {
			return nil, errors.New("failed to read image file")
		}

		mimeType := http.DetectContentType(imageData)
		base64Data := base64.StdEncoding.EncodeToString(imageData)
		imageBase64s = append(imageBase64s, fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data))
	}
	return imageBase64s, nil
}

func oaiFormEdit2AliImageEdit(c *gin.Context, _ *relaycommon.RelayInfo, request dto.ImageRequest) (*AliImageRequest, error) {
	var imageRequest AliImageRequest
	imageRequest.Model = request.Model
	imageRequest.ResponseFormat = request.ResponseFormat

	imageBase64s, err := getImageBase64sFromForm(c, "image")
	if err != nil {
		return nil, fmt.Errorf("get image base64s from form failed: %w", err)
	}
	mediaContents := make([]AliMediaContent, len(imageBase64s))
	for i, b64 := range imageBase64s {
		mediaContents[i] = AliMediaContent{Image: b64}
	}
	mediaContents = append(mediaContents, AliMediaContent{Text: request.Prompt})

	imageRequest.Input = AliImageInput{
		Messages: []AliMessage{{
			Role:    "user",
			Content: mediaContents,
		}},
	}
	imageRequest.Parameters = AliImageParameters{
		N:         int(lo.FromPtrOr(request.N, uint(1))),
		Watermark: request.Watermark,
	}
	return &imageRequest, nil
}

func updateTask(info *relaycommon.RelayInfo, taskID string) (*AliResponse, error, []byte) {
	url := fmt.Sprintf("%s/api/v1/tasks/%s", info.ChannelBaseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return &AliResponse{}, err, nil
	}
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		platformobservability.SysLog("updateTask client.Do err: " + err.Error())
		return &AliResponse{}, err, nil
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &AliResponse{}, err, nil
	}

	var response AliResponse
	if err := platformencoding.Unmarshal(responseBody, &response); err != nil {
		platformobservability.SysLog("updateTask NewDecoder err: " + err.Error())
		return &AliResponse{}, err, nil
	}
	return &response, nil, responseBody
}

func asyncTaskWait(c *gin.Context, info *relaycommon.RelayInfo, taskID string) (*AliResponse, []byte, error) {
	waitSeconds := 10
	step := 0
	maxStep := 20

	var responseBody []byte
	time.Sleep(5 * time.Second)

	for {
		logger.LogDebug(c, fmt.Sprintf("asyncTaskWait step %d/%d, wait %d seconds", step, maxStep, waitSeconds))
		step++

		rsp, err, body := updateTask(info, taskID)
		responseBody = body
		if err != nil {
			logger.LogWarn(c, "asyncTaskWait UpdateTask err: "+err.Error())
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		}
		if rsp.Output.TaskStatus == "" {
			return &AliResponse{}, responseBody, nil
		}

		switch rsp.Output.TaskStatus {
		case "FAILED", "CANCELED", "SUCCEEDED", "UNKNOWN":
			return rsp, responseBody, nil
		}
		if step >= maxStep {
			break
		}
		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}

	return nil, nil, fmt.Errorf("aliAsyncTaskWait timeout")
}

func responseAli2OpenAIImage(c *gin.Context, response *AliResponse, originBody []byte, info *relaycommon.RelayInfo, responseFormat string) *dto.ImageResponse {
	imageResponse := dto.ImageResponse{Created: info.StartTime.Unix()}
	if len(response.Output.Results) > 0 {
		imageResponse.Data = response.Output.ResultToOpenAIImageDate(c, responseFormat)
	} else if len(response.Output.Choices) > 0 {
		imageResponse.Data = response.Output.ChoicesToOpenAIImageDate(c, responseFormat)
	}
	imageResponse.Metadata = originBody
	return &imageResponse
}

func aliImageHandler(a *Adaptor, c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*types.APIError, *dto.Usage) {
	responseFormat := c.GetString("response_format")

	var aliTaskResponse AliResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}
	platformhttpx.CloseResponseBodyGracefully(resp)
	if err := platformencoding.Unmarshal(responseBody, &aliTaskResponse); err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}
	if aliTaskResponse.Message != "" {
		logger.LogError(c, "ali_async_task_failed: "+aliTaskResponse.Message)
		return types.NewError(errors.New(aliTaskResponse.Message), types.ErrorCodeBadResponse), nil
	}

	var (
		aliResponse    *AliResponse
		originRespBody []byte
	)
	if a.IsSyncImageModel {
		aliResponse = &aliTaskResponse
		originRespBody = responseBody
	} else {
		aliResponse, originRespBody, err = asyncTaskWait(c, info, aliTaskResponse.Output.TaskId)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponse), nil
		}
		if aliResponse.Output.TaskStatus != "SUCCEEDED" {
			return types.WithOpenAIError(types.OpenAIError{
				Message: aliResponse.Output.Message,
				Type:    "ali_error",
				Code:    aliResponse.Output.Code,
			}, resp.StatusCode), nil
		}
	}

	if a.IsSyncImageModel {
		logger.LogDebug(c, "ali_sync_image_result: "+string(originRespBody))
	} else {
		logger.LogDebug(c, "ali_async_image_result: "+string(originRespBody))
	}

	imageResponses := responseAli2OpenAIImage(c, aliResponse, originRespBody, info, responseFormat)
	if aliResponse.Usage.ImageCount != 0 {
		info.PriceData.AddOtherRatio("n", float64(aliResponse.Usage.ImageCount))
	} else if len(imageResponses.Data) != 0 {
		info.PriceData.AddOtherRatio("n", float64(len(imageResponses.Data)))
	}

	jsonResponse, err := platformencoding.Marshal(imageResponses)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	platformhttpx.IOCopyBytesGracefully(c, resp, jsonResponse)
	return nil, &dto.Usage{}
}
