package stream

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"net/http"
)

func markResponseBodyDelivered(c *gin.Context) {
	if c == nil {
		return
	}
	httpctx.SetContextKey(c, constant.ContextKeyResponseBodyDelivered, true)
}

func FlushWriter(c *gin.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("flush panic recovered: %v", r)
		}
	}()

	if c == nil || c.Writer == nil {
		return nil
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return errors.New("streaming error: flusher not found")
	}
	flusher.Flush()
	return nil
}

func SetEventStreamHeaders(c *gin.Context) {
	if _, exists := c.Get("event_stream_headers_set"); exists {
		return
	}
	c.Set("event_stream_headers_set", true)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func IsClientGone(c *gin.Context) bool {
	return c == nil || c.Request == nil || c.Request.Context().Err() != nil
}

func ClaudeData(c *gin.Context, resp dto.ClaudeResponse) error {
	if IsClientGone(c) {
		return fmt.Errorf("request context done")
	}
	jsonData, err := platformencoding.Marshal(resp)
	if err == nil {
		c.Render(-1, CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
		c.Render(-1, CustomEvent{Data: "data: " + string(jsonData)})
	}
	err = FlushWriter(c)
	if err == nil {
		markResponseBodyDelivered(c)
	}
	return err
}

func ClaudeChunkData(c *gin.Context, resp dto.ClaudeResponse, data string) error {
	if IsClientGone(c) {
		return fmt.Errorf("request context done")
	}
	c.Render(-1, CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
	c.Render(-1, CustomEvent{Data: fmt.Sprintf("data: %s\n", data)})
	err := FlushWriter(c)
	if err == nil {
		markResponseBodyDelivered(c)
	}
	return err
}

func ResponseChunkData(c *gin.Context, resp dto.ResponsesStreamResponse, data string) error {
	if IsClientGone(c) {
		return fmt.Errorf("request context done")
	}
	c.Render(-1, CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
	c.Render(-1, CustomEvent{Data: fmt.Sprintf("data: %s", data)})
	err := FlushWriter(c)
	if err == nil {
		markResponseBodyDelivered(c)
	}
	return err
}

func StringData(c *gin.Context, str string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}
	c.Render(-1, CustomEvent{Data: "data: " + str})
	err := FlushWriter(c)
	if err == nil && str != "[DONE]" {
		markResponseBodyDelivered(c)
	}
	return err
}

func PingData(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}
	if _, err := c.Writer.Write([]byte(": PING\n\n")); err != nil {
		return fmt.Errorf("write ping data failed: %w", err)
	}
	return FlushWriter(c)
}

func ObjectData(c *gin.Context, object interface{}) error {
	if object == nil {
		return errors.New("object is nil")
	}
	jsonData, err := platformencoding.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	return StringData(c, string(jsonData))
}

func Done(c *gin.Context) {
	if IsClientGone(c) {
		return
	}
	_ = StringData(c, "[DONE]")
}

func WssString(c *gin.Context, ws *websocket.Conn, str string) error {
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	return ws.WriteMessage(websocket.TextMessage, []byte(str))
}

func WssObject(c *gin.Context, ws *websocket.Conn, object interface{}) error {
	jsonData, err := platformencoding.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	return ws.WriteMessage(websocket.TextMessage, jsonData)
}

func WssError(c *gin.Context, ws *websocket.Conn, openaiError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openaiError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetResponseID(c *gin.Context) string {
	logID := c.GetString(constant.RequestIdKey)
	return fmt.Sprintf("chatcmpl-%s", logID)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(constant.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}

func GenerateStartEmptyResponse(id string, createAt int64, model string, systemFingerprint *string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: platformruntime.GetPointer(""),
				},
			},
		},
	}
}

func GenerateStopResponse(id string, createAt int64, model string, finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: createAt,
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func GenerateFinalUsageResponse(id string, createAt int64, model string, usage dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: createAt,
		Model:   model,
		Choices: make([]dto.ChatCompletionsStreamResponseChoice, 0),
		Usage:   &usage,
	}
}
