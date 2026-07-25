package dify

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

func uploadDifyFile(c *gin.Context, info *relaycommon.RelayInfo, user string, media dto.MediaContent) *DifyFile {
	uploadURL := fmt.Sprintf("%s/v1/files/upload", info.ChannelBaseUrl)
	switch media.Type {
	case dto.ContentTypeImageURL:
		imageMedia := media.GetImageMedia()
		base64Data := imageMedia.Url
		if idx := strings.Index(base64Data, ","); idx != -1 {
			base64Data = base64Data[idx+1:]
		}

		decodedData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			platformobservability.SysLog("failed to decode base64: " + err.Error())
			return nil
		}

		tempFile, err := os.CreateTemp("", "dify-upload-*")
		if err != nil {
			platformobservability.SysLog("failed to create temp file: " + err.Error())
			return nil
		}
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		if _, err := tempFile.Write(decodedData); err != nil {
			platformobservability.SysLog("failed to write to temp file: " + err.Error())
			return nil
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		if err := writer.WriteField("user", user); err != nil {
			platformobservability.SysLog("failed to add user field: " + err.Error())
			return nil
		}

		mimeType := imageMedia.MimeType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}

		part, err := writer.CreateFormFile("file", fmt.Sprintf("image.%s", strings.TrimPrefix(mimeType, "image/")))
		if err != nil {
			platformobservability.SysLog("failed to create form file: " + err.Error())
			return nil
		}
		if _, err = io.Copy(part, bytes.NewReader(decodedData)); err != nil {
			platformobservability.SysLog("failed to copy file content: " + err.Error())
			return nil
		}
		_ = writer.Close()

		req, err := http.NewRequest(http.MethodPost, uploadURL, body)
		if err != nil {
			platformobservability.SysLog("failed to create request: " + err.Error())
			return nil
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))

		resp, err := platformhttpx.GetHTTPClient().Do(req)
		if err != nil {
			platformobservability.SysLog("failed to send request: " + err.Error())
			return nil
		}
		defer resp.Body.Close()

		var result struct {
			Id string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			platformobservability.SysLog("failed to decode response: " + err.Error())
			return nil
		}

		return &DifyFile{
			UploadFileId: result.Id,
			Type:         "image",
			TransferMode: "local_file",
		}
	}
	return nil
}

func requestOpenAI2Dify(c *gin.Context, info *relaycommon.RelayInfo, request dto.GeneralOpenAIRequest) *DifyChatRequest {
	difyReq := DifyChatRequest{
		Inputs:           make(map[string]interface{}),
		AutoGenerateName: false,
	}

	user := request.User
	if len(user) == 0 {
		user = json.RawMessage(gatewaystream.GetResponseID(c))
	}
	var stringUser string
	if err := json.Unmarshal(user, &stringUser); err != nil {
		platformobservability.SysLog("failed to unmarshal user: " + err.Error())
		stringUser = gatewaystream.GetResponseID(c)
	}
	difyReq.User = stringUser

	files := make([]DifyFile, 0)
	var content strings.Builder
	for _, message := range request.Messages {
		if message.Role == "system" {
			content.WriteString("SYSTEM: \n" + message.StringContent() + "\n")
			continue
		}
		if message.Role == "assistant" {
			content.WriteString("ASSISTANT: \n" + message.StringContent() + "\n")
			continue
		}

		for _, mediaContent := range message.ParseContent() {
			switch mediaContent.Type {
			case dto.ContentTypeText:
				content.WriteString("USER: \n" + mediaContent.Text + "\n")
			case dto.ContentTypeImageURL:
				media := mediaContent.GetImageMedia()
				var file *DifyFile
				if media.IsRemoteImage() {
					file = &DifyFile{
						Type:         "image",
						TransferMode: "remote_url",
						URL:          media.Url,
					}
				} else {
					file = uploadDifyFile(c, info, difyReq.User, mediaContent)
				}
				if file != nil {
					files = append(files, *file)
				}
			}
		}
	}

	difyReq.Query = content.String()
	difyReq.Files = files
	if lo.FromPtrOr(request.Stream, false) {
		difyReq.ResponseMode = "streaming"
	} else {
		difyReq.ResponseMode = "blocking"
	}
	return &difyReq
}

func streamResponseDify2OpenAI(difyResponse DifyChunkChatCompletionResponse) *dto.ChatCompletionsStreamResponse {
	response := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Created: platformruntime.GetTimestamp(),
		Model:   "dify",
	}

	var choice dto.ChatCompletionsStreamResponseChoice
	if strings.HasPrefix(difyResponse.Event, "workflow_") {
		if constant.DifyDebug {
			text := "Workflow: " + difyResponse.Data.WorkflowId
			if difyResponse.Event == "workflow_finished" {
				text += " " + difyResponse.Data.Status
			}
			choice.Delta.SetReasoningContent(text + "\n")
		}
	} else if strings.HasPrefix(difyResponse.Event, "node_") {
		if constant.DifyDebug {
			text := "Node: " + difyResponse.Data.NodeType
			if difyResponse.Event == "node_finished" {
				text += " " + difyResponse.Data.Status
			}
			choice.Delta.SetReasoningContent(text + "\n")
		}
	} else if difyResponse.Event == "message" || difyResponse.Event == "agent_message" {
		if difyResponse.Answer == "<details style=\"color:gray;background-color: #f8f8f8;padding: 8px;border-radius: 4px;\" open> <summary> Thinking... </summary>\n" {
			difyResponse.Answer = "<think>"
		} else if difyResponse.Answer == "</details>" {
			difyResponse.Answer = "</think>"
		}
		choice.Delta.SetContentString(difyResponse.Answer)
	}

	response.Choices = append(response.Choices, choice)
	return &response
}

func difyStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	var responseText string
	usage := &dto.Usage{}
	nodeToken := 0

	gatewaystream.SetEventStreamHeaders(c)
	var streamErr *types.APIError
	gatewaystream.ScanResponse(c, resp, info, func(data string, sr *gatewaystream.Result) {
		var difyResponse DifyChunkChatCompletionResponse
		if err := json.Unmarshal([]byte(data), &difyResponse); err != nil {
			platformobservability.SysLog("error unmarshalling stream response: " + err.Error())
			sr.Error(err)
			streamErr = types.NewError(err, types.ErrorCodeBadResponseBody)
			return
		}
		if difyResponse.Event == "message_end" {
			usage = &difyResponse.MetaData.Usage
			sr.Done()
			return
		}
		if difyResponse.Event == "error" {
			streamErr = types.NewError(fmt.Errorf("dify error event"), types.ErrorCodeBadResponse)
			sr.Stop(streamErr)
			return
		}

		openAIResponse := streamResponseDify2OpenAI(difyResponse)
		if len(openAIResponse.Choices) != 0 {
			responseText += openAIResponse.Choices[0].Delta.GetContentString()
			if openAIResponse.Choices[0].Delta.ReasoningContent != nil {
				nodeToken++
			}
		}
		if err := gatewaystream.ObjectData(c, openAIResponse); err != nil {
			platformobservability.SysLog(err.Error())
			sr.Error(err)
			streamErr = types.NewError(err, types.ErrorCodeDoRequestFailed)
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}
	gatewaystream.Done(c)
	if usage.TotalTokens == 0 {
		usage = tokenx.ResponseText2Usage(c, responseText, info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	usage.CompletionTokens += nodeToken
	return usage, nil
}

func difyHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	var difyResponse DifyChatCompletionResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	platformhttpx.CloseResponseBodyGracefully(resp)
	if err := json.Unmarshal(responseBody, &difyResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	fullTextResponse := dto.OpenAITextResponse{
		Id:      difyResponse.ConversationId,
		Object:  "chat.completion",
		Created: platformruntime.GetTimestamp(),
		Usage:   difyResponse.MetaData.Usage,
	}
	choice := dto.OpenAITextResponseChoice{
		Index: 0,
		Message: dto.Message{
			Role:    "assistant",
			Content: difyResponse.Answer,
		},
		FinishReason: "stop",
	}
	fullTextResponse.Choices = append(fullTextResponse.Choices, choice)
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &difyResponse.MetaData.Usage, nil
}
