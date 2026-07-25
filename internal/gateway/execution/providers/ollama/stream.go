package ollama

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
	"time"
)

type ollamaChatStreamChunk struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   *struct {
		Role      string          `json:"role"`
		Content   string          `json:"content"`
		Thinking  json.RawMessage `json:"thinking"`
		ToolCalls []struct {
			Function struct {
				Name      string      `json:"name"`
				Arguments interface{} `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	EvalCount          int    `json:"eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalDuration       int64  `json:"eval_duration"`
}

func toUnix(ts string) int64 {
	if ts == "" {
		return time.Now().Unix()
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t2, err2 := time.Parse(time.RFC3339, ts)
		if err2 == nil {
			return t2.Unix()
		}
		return time.Now().Unix()
	}
	return t.Unix()
}

func ollamaStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("empty response"), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}
	defer platformhttpx.CloseResponseBodyGracefully(resp)

	gatewaystream.SetEventStreamHeaders(c)
	scanner := gatewaystream.NewStreamScanner(resp.Body)
	usage := &dto.Usage{}
	model := info.UpstreamModelName
	responseID := platformruntime.GetUUID()
	created := time.Now().Unix()
	var toolCallIndex int
	start := gatewaystream.GenerateStartEmptyResponse(responseID, created, model, nil)
	if data, err := platformencoding.Marshal(start); err == nil {
		if err = gatewaystream.StringData(c, string(data)); err != nil {
			logger.LogError(c, "ollama stream write start error: "+err.Error())
			return usage, nil
		}
	}

	for scanner.Scan() {
		if gatewaystream.IsClientGone(c) {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk ollamaChatStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			logger.LogError(c, "ollama stream json decode error: "+err.Error()+" line="+line)
			return usage, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		created = toUnix(chunk.CreatedAt)
		if !chunk.Done {
			var content string
			if chunk.Message != nil {
				content = chunk.Message.Content
			} else {
				content = chunk.Response
			}
			delta := dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"},
				}},
			}
			if content != "" {
				delta.Choices[0].Delta.SetContentString(content)
			}
			if chunk.Message != nil && len(chunk.Message.Thinking) > 0 {
				raw := strings.TrimSpace(string(chunk.Message.Thinking))
				if raw != "" && raw != "null" {
					var thinkingContent string
					if err := json.Unmarshal(chunk.Message.Thinking, &thinkingContent); err == nil {
						delta.Choices[0].Delta.SetReasoningContent(thinkingContent)
					} else {
						delta.Choices[0].Delta.SetReasoningContent(raw)
					}
				}
			}
			if chunk.Message != nil && len(chunk.Message.ToolCalls) > 0 {
				delta.Choices[0].Delta.ToolCalls = make([]dto.ToolCallResponse, 0, len(chunk.Message.ToolCalls))
				for _, tc := range chunk.Message.ToolCalls {
					argBytes, _ := json.Marshal(tc.Function.Arguments)
					toolID := fmt.Sprintf("call_%d", toolCallIndex)
					tr := dto.ToolCallResponse{
						ID:   toolID,
						Type: "function",
						Function: dto.FunctionResponse{
							Name:      tc.Function.Name,
							Arguments: string(argBytes),
						},
					}
					tr.SetIndex(toolCallIndex)
					toolCallIndex++
					delta.Choices[0].Delta.ToolCalls = append(delta.Choices[0].Delta.ToolCalls, tr)
				}
			}
			if data, err := platformencoding.Marshal(delta); err == nil {
				if err = gatewaystream.StringData(c, string(data)); err != nil {
					logger.LogError(c, "ollama stream write delta error: "+err.Error())
					break
				}
			}
			continue
		}
		usage.PromptTokens = chunk.PromptEvalCount
		usage.CompletionTokens = chunk.EvalCount
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		finishReason := chunk.DoneReason
		if finishReason == "" {
			finishReason = "stop"
		}
		if stop := gatewaystream.GenerateStopResponse(responseID, created, model, finishReason); stop != nil {
			if data, err := platformencoding.Marshal(stop); err == nil {
				if err = gatewaystream.StringData(c, string(data)); err != nil {
					logger.LogError(c, "ollama stream write stop error: "+err.Error())
					break
				}
			}
		}
		if final := gatewaystream.GenerateFinalUsageResponse(responseID, created, model, *usage); final != nil {
			if data, err := platformencoding.Marshal(final); err == nil {
				if err = gatewaystream.StringData(c, string(data)); err != nil {
					logger.LogError(c, "ollama stream write usage error: "+err.Error())
					break
				}
			}
		}
		gatewaystream.Done(c)
		break
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		logger.LogError(c, "ollama stream scan error: "+err.Error())
	}
	return usage, nil
}

func ollamaChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	platformhttpx.CloseResponseBodyGracefully(resp)
	raw := string(body)
	if platformconfig.DebugEnabled {
		println("ollama non-stream raw resp:", raw)
	}

	lines := strings.Split(raw, "\n")
	var (
		aggContent       strings.Builder
		reasoningBuilder strings.Builder
		lastChunk        ollamaChatStreamChunk
		parsedAny        bool
	)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var ck ollamaChatStreamChunk
		if err := json.Unmarshal([]byte(ln), &ck); err != nil {
			if len(lines) == 1 {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			continue
		}
		parsedAny = true
		lastChunk = ck
		if ck.Message != nil && len(ck.Message.Thinking) > 0 {
			rawThinking := strings.TrimSpace(string(ck.Message.Thinking))
			if rawThinking != "" && rawThinking != "null" {
				var thinkingContent string
				if err := json.Unmarshal(ck.Message.Thinking, &thinkingContent); err == nil {
					reasoningBuilder.WriteString(thinkingContent)
				} else {
					reasoningBuilder.WriteString(rawThinking)
				}
			}
		}
		if ck.Message != nil && ck.Message.Content != "" {
			aggContent.WriteString(ck.Message.Content)
		} else if ck.Response != "" {
			aggContent.WriteString(ck.Response)
		}
	}

	if !parsedAny {
		var single ollamaChatStreamChunk
		if err := json.Unmarshal(body, &single); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		lastChunk = single
		if single.Message != nil {
			if len(single.Message.Thinking) > 0 {
				rawThinking := strings.TrimSpace(string(single.Message.Thinking))
				if rawThinking != "" && rawThinking != "null" {
					var thinkingContent string
					if err := json.Unmarshal(single.Message.Thinking, &thinkingContent); err == nil {
						reasoningBuilder.WriteString(thinkingContent)
					} else {
						reasoningBuilder.WriteString(rawThinking)
					}
				}
			}
			aggContent.WriteString(single.Message.Content)
		} else {
			aggContent.WriteString(single.Response)
		}
	}

	model := lastChunk.Model
	if model == "" {
		model = info.UpstreamModelName
	}
	created := toUnix(lastChunk.CreatedAt)
	usage := &dto.Usage{
		PromptTokens:     lastChunk.PromptEvalCount,
		CompletionTokens: lastChunk.EvalCount,
		TotalTokens:      lastChunk.PromptEvalCount + lastChunk.EvalCount,
	}
	content := aggContent.String()
	finishReason := lastChunk.DoneReason
	if finishReason == "" {
		finishReason = "stop"
	}
	msg := dto.Message{Role: "assistant", Content: contentPtr(content)}
	if rc := reasoningBuilder.String(); rc != "" {
		msg.ReasoningContent = &rc
	}
	full := dto.OpenAITextResponse{
		Id:      platformruntime.GetUUID(),
		Model:   model,
		Object:  "chat.completion",
		Created: created,
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	out, _ := platformencoding.Marshal(full)
	platformhttpx.IOCopyBytesGracefully(c, resp, out)
	return usage, nil
}

func contentPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
