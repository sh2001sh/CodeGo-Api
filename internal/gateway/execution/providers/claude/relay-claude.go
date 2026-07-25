package claude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/reasoning"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformfilex "github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"strings"
)

const (
	WebSearchMaxUsesLow    = 1
	WebSearchMaxUsesMedium = 5
	WebSearchMaxUsesHigh   = 10
)

type requestReasoning struct {
	Enabled   bool   `json:"enabled"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Exclude   bool   `json:"exclude,omitempty"`
}

func openAIFileToClaudeMediaMessage(c *gin.Context, source types.FileSource) (*dto.ClaudeMediaMessage, error) {
	base64Data, mimeType, err := platformfilex.GetBase64Data(c, source, "formatting file for Claude")
	if err != nil {
		return nil, fmt.Errorf("get file data failed: %s", err.Error())
	}

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return &dto.ClaudeMediaMessage{
			Type: "image",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: mimeType,
				Data:      base64Data,
			},
		}, nil
	case strings.HasPrefix(mimeType, "application/pdf"):
		return &dto.ClaudeMediaMessage{
			Type: "document",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: mimeType,
				Data:      base64Data,
			},
		}, nil
	case strings.HasPrefix(mimeType, "text/"):
		decoded, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("decode text file failed: %s", err.Error())
		}
		return &dto.ClaudeMediaMessage{
			Type: "text",
			Text: platformruntime.GetPointer(string(decoded)),
		}, nil
	default:
		return nil, nil
	}
}

func maybeMarkClaudeRefusal(c *gin.Context, stopReason string) {
	if c != nil && strings.EqualFold(stopReason, "refusal") {
		httpctx.SetContextKey(c, constant.ContextKeyAdminRejectReason, "claude_stop_reason=refusal")
	}
}

func RequestOpenAI2ClaudeMessage(c *gin.Context, textRequest dto.GeneralOpenAIRequest) (*dto.ClaudeRequest, error) {
	claudeTools := make([]any, 0, len(textRequest.Tools))
	for _, tool := range textRequest.Tools {
		if params, ok := tool.Function.Parameters.(map[string]any); ok {
			claudeTool := dto.Tool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: map[string]interface{}{},
			}
			if params["type"] != nil {
				claudeTool.InputSchema["type"] = params["type"].(string)
			}
			claudeTool.InputSchema["properties"] = params["properties"]
			claudeTool.InputSchema["required"] = params["required"]
			for s, a := range params {
				if s == "type" || s == "properties" || s == "required" {
					continue
				}
				claudeTool.InputSchema[s] = a
			}
			claudeTools = append(claudeTools, &claudeTool)
		}
	}

	if textRequest.WebSearchOptions != nil {
		webSearchTool := dto.ClaudeWebSearchTool{
			Type: "web_search_20250305",
			Name: "web_search",
		}
		if textRequest.WebSearchOptions.UserLocation != nil {
			anthropicUserLocation := &dto.ClaudeWebSearchUserLocation{Type: "approximate"}
			var userLocationMap map[string]interface{}
			if err := platformencoding.Unmarshal(textRequest.WebSearchOptions.UserLocation, &userLocationMap); err == nil {
				if approximateData, ok := userLocationMap["approximate"].(map[string]interface{}); ok {
					if timezone, ok := approximateData["timezone"].(string); ok && timezone != "" {
						anthropicUserLocation.Timezone = timezone
					}
					if country, ok := approximateData["country"].(string); ok && country != "" {
						anthropicUserLocation.Country = country
					}
					if region, ok := approximateData["region"].(string); ok && region != "" {
						anthropicUserLocation.Region = region
					}
					if city, ok := approximateData["city"].(string); ok && city != "" {
						anthropicUserLocation.City = city
					}
				}
			}
			webSearchTool.UserLocation = anthropicUserLocation
		}
		switch textRequest.WebSearchOptions.SearchContextSize {
		case "low":
			webSearchTool.MaxUses = WebSearchMaxUsesLow
		case "medium":
			webSearchTool.MaxUses = WebSearchMaxUsesMedium
		case "high":
			webSearchTool.MaxUses = WebSearchMaxUsesHigh
		}
		claudeTools = append(claudeTools, &webSearchTool)
	}

	claudeRequest := dto.ClaudeRequest{
		Model:       textRequest.Model,
		Temperature: textRequest.Temperature,
		Tools:       claudeTools,
	}
	if maxTokens := textRequest.GetMaxTokens(); maxTokens > 0 {
		claudeRequest.MaxTokens = platformruntime.GetPointer(maxTokens)
	}
	if textRequest.TopP != nil {
		claudeRequest.TopP = platformruntime.GetPointer(*textRequest.TopP)
	}
	if textRequest.TopK != nil {
		claudeRequest.TopK = platformruntime.GetPointer(*textRequest.TopK)
	}
	if textRequest.IsStream(nil) {
		claudeRequest.Stream = platformruntime.GetPointer(true)
	}
	if textRequest.ToolChoice != nil || textRequest.ParallelTooCalls != nil {
		if claudeToolChoice := mapToolChoice(textRequest.ToolChoice, textRequest.ParallelTooCalls); claudeToolChoice != nil {
			claudeRequest.ToolChoice = claudeToolChoice
		}
	}
	if claudeRequest.MaxTokens == nil || *claudeRequest.MaxTokens == 0 {
		defaultMaxTokens := uint(gatewaystore.GetClaudeSettings().GetDefaultMaxTokens(textRequest.Model))
		claudeRequest.MaxTokens = &defaultMaxTokens
	}

	if baseModel, effortLevel, ok := reasoning.TrimEffortSuffix(textRequest.Model); ok && effortLevel != "" &&
		(strings.HasPrefix(textRequest.Model, "claude-opus-4-6") ||
			strings.HasPrefix(textRequest.Model, "claude-opus-4-7") ||
			strings.HasPrefix(textRequest.Model, "claude-opus-4-8")) {
		claudeRequest.Model = baseModel
		claudeRequest.Thinking = &dto.Thinking{Type: "adaptive"}
		claudeRequest.OutputConfig = json.RawMessage(fmt.Sprintf(`{"effort":"%s"}`, effortLevel))
		if strings.HasPrefix(baseModel, "claude-opus-4-7") || strings.HasPrefix(baseModel, "claude-opus-4-8") {
			claudeRequest.Thinking.Display = "summarized"
			claudeRequest.Temperature = nil
			claudeRequest.TopP = nil
			claudeRequest.TopK = nil
		} else {
			claudeRequest.TopP = nil
			claudeRequest.Temperature = platformruntime.GetPointer[float64](1.0)
		}
	} else if gatewaystore.GetClaudeSettings().ThinkingAdapterEnabled &&
		strings.HasSuffix(textRequest.Model, "-thinking") {
		trimmedModel := strings.TrimSuffix(textRequest.Model, "-thinking")
		if strings.HasPrefix(trimmedModel, "claude-opus-4-7") || strings.HasPrefix(trimmedModel, "claude-opus-4-8") {
			claudeRequest.Thinking = &dto.Thinking{Type: "adaptive", Display: "summarized"}
			claudeRequest.OutputConfig = json.RawMessage(`{"effort":"high"}`)
			claudeRequest.Temperature = nil
			claudeRequest.TopP = nil
			claudeRequest.TopK = nil
		} else {
			if claudeRequest.MaxTokens == nil || *claudeRequest.MaxTokens < 1280 {
				claudeRequest.MaxTokens = platformruntime.GetPointer[uint](1280)
			}
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: platformruntime.GetPointer[int](int(float64(*claudeRequest.MaxTokens) * gatewaystore.GetClaudeSettings().ThinkingAdapterBudgetTokensPercentage)),
			}
			claudeRequest.TopP = nil
			claudeRequest.Temperature = platformruntime.GetPointer[float64](1.0)
		}
		if !gatewaystore.ShouldPreserveThinkingSuffix(textRequest.Model) {
			claudeRequest.Model = trimmedModel
		}
	}

	switch textRequest.ReasoningEffort {
	case "low":
		claudeRequest.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: platformruntime.GetPointer[int](1280)}
	case "medium":
		claudeRequest.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: platformruntime.GetPointer[int](2048)}
	case "high":
		claudeRequest.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: platformruntime.GetPointer[int](4096)}
	}

	if textRequest.Reasoning != nil {
		var reasoningReq requestReasoning
		if err := platformencoding.Unmarshal(textRequest.Reasoning, &reasoningReq); err != nil {
			return nil, err
		}
		if reasoningReq.MaxTokens > 0 {
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: &reasoningReq.MaxTokens,
			}
		}
	}

	if textRequest.Stop != nil {
		switch v := textRequest.Stop.(type) {
		case string:
			claudeRequest.StopSequences = []string{v}
		case []interface{}:
			stopSequences := make([]string, 0, len(v))
			for _, stop := range v {
				stopSequences = append(stopSequences, stop.(string))
			}
			claudeRequest.StopSequences = stopSequences
		}
	}

	formatMessages := make([]dto.Message, 0, len(textRequest.Messages))
	lastMessage := dto.Message{Role: "tool"}
	for i, message := range textRequest.Messages {
		if message.Role == "" {
			textRequest.Messages[i].Role = "user"
		}
		fmtMessage := dto.Message{
			Role:    message.Role,
			Content: message.Content,
		}
		if message.Role == "tool" {
			fmtMessage.ToolCallId = message.ToolCallId
		}
		if message.Role == "assistant" && message.ToolCalls != nil {
			fmtMessage.ToolCalls = message.ToolCalls
		}
		if lastMessage.Role == message.Role && lastMessage.Role != "tool" && lastMessage.IsStringContent() && message.IsStringContent() {
			fmtMessage.SetStringContent(strings.Trim(fmt.Sprintf("%s %s", lastMessage.StringContent(), message.StringContent()), "\""))
			formatMessages = formatMessages[:len(formatMessages)-1]
		}
		if fmtMessage.Content == nil || (fmtMessage.IsStringContent() && fmtMessage.StringContent() == "") {
			fmtMessage.SetStringContent("...")
		}
		formatMessages = append(formatMessages, fmtMessage)
		lastMessage = fmtMessage
	}

	claudeMessages := make([]dto.ClaudeMessage, 0)
	isFirstMessage := true
	var systemMessages []dto.ClaudeMediaMessage

	for _, message := range formatMessages {
		if message.Role == "system" {
			if message.IsStringContent() {
				if text := message.StringContent(); text != "" {
					systemMessages = append(systemMessages, dto.ClaudeMediaMessage{Type: "text", Text: platformruntime.GetPointer[string](text)})
				}
			} else {
				for _, ctx := range message.ParseContent() {
					if ctx.Type == "text" && ctx.Text != "" {
						systemMessages = append(systemMessages, dto.ClaudeMediaMessage{Type: "text", Text: platformruntime.GetPointer[string](ctx.Text)})
					}
				}
			}
			continue
		}

		if isFirstMessage {
			isFirstMessage = false
			if message.Role != "user" {
				claudeMessages = append(claudeMessages, dto.ClaudeMessage{
					Role: "user",
					Content: []dto.ClaudeMediaMessage{{
						Type: "text",
						Text: platformruntime.GetPointer[string]("..."),
					}},
				})
			}
		}

		claudeMessage := dto.ClaudeMessage{Role: message.Role}
		if message.Role == "tool" {
			if len(claudeMessages) > 0 && claudeMessages[len(claudeMessages)-1].Role == "user" {
				lastMessage := claudeMessages[len(claudeMessages)-1]
				if content, ok := lastMessage.Content.(string); ok {
					lastMessage.Content = []dto.ClaudeMediaMessage{{Type: "text", Text: platformruntime.GetPointer[string](content)}}
				}
				lastMessage.Content = append(lastMessage.Content.([]dto.ClaudeMediaMessage), dto.ClaudeMediaMessage{
					Type:      "tool_result",
					ToolUseId: message.ToolCallId,
					Content:   message.Content,
				})
				claudeMessages[len(claudeMessages)-1] = lastMessage
				continue
			}
			claudeMessage.Role = "user"
			claudeMessage.Content = []dto.ClaudeMediaMessage{{
				Type:      "tool_result",
				ToolUseId: message.ToolCallId,
				Content:   message.Content,
			}}
		} else if message.IsStringContent() && message.ToolCalls == nil {
			text := message.StringContent()
			if text == "" {
				text = "..."
			}
			claudeMessage.Content = text
		} else {
			claudeMediaMessages := make([]dto.ClaudeMediaMessage, 0)
			for _, mediaMessage := range message.ParseContent() {
				switch mediaMessage.Type {
				case "text":
					if mediaMessage.Text != "" {
						claudeMediaMessages = append(claudeMediaMessages, dto.ClaudeMediaMessage{
							Type: "text",
							Text: platformruntime.GetPointer[string](mediaMessage.Text),
						})
					}
				default:
					source := mediaMessage.ToFileSource()
					if source == nil {
						continue
					}
					claudeMediaMessage, err := openAIFileToClaudeMediaMessage(c, source)
					if err != nil {
						return nil, err
					}
					if claudeMediaMessage != nil {
						claudeMediaMessages = append(claudeMediaMessages, *claudeMediaMessage)
					}
				}
			}
			if message.ToolCalls != nil {
				for _, toolCall := range message.ParseToolCalls() {
					inputObj := make(map[string]any)
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &inputObj); err != nil {
						platformobservability.SysLog("tool call function arguments is not a map[string]any: " + fmt.Sprintf("%v", toolCall.Function.Arguments))
						continue
					}
					claudeMediaMessages = append(claudeMediaMessages, dto.ClaudeMediaMessage{
						Type:  "tool_use",
						Id:    toolCall.ID,
						Name:  toolCall.Function.Name,
						Input: inputObj,
					})
				}
			}
			claudeMessage.Content = claudeMediaMessages
		}
		claudeMessages = append(claudeMessages, claudeMessage)
	}

	if len(systemMessages) > 0 {
		claudeRequest.System = systemMessages
	}
	claudeRequest.Messages = claudeMessages
	return &claudeRequest, nil
}

func StreamResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.ChatCompletionsStreamResponse {
	var response dto.ChatCompletionsStreamResponse
	response.Object = "chat.completion.chunk"
	response.Model = claudeResponse.Model
	response.Choices = make([]dto.ChatCompletionsStreamResponseChoice, 0)
	tools := make([]dto.ToolCallResponse, 0)
	fcIdx := 0
	if claudeResponse.Index != nil {
		fcIdx = *claudeResponse.Index - 1
		if fcIdx < 0 {
			fcIdx = 0
		}
	}
	var choice dto.ChatCompletionsStreamResponseChoice
	switch claudeResponse.Type {
	case "message_start":
		if claudeResponse.Message != nil {
			response.Id = claudeResponse.Message.Id
			response.Model = claudeResponse.Message.Model
		}
		choice.Delta.SetContentString("")
		choice.Delta.Role = "assistant"
	case "content_block_start":
		if claudeResponse.ContentBlock == nil {
			return nil
		}
		if claudeResponse.ContentBlock.Type == "text" && claudeResponse.ContentBlock.Text != nil {
			choice.Delta.SetContentString(*claudeResponse.ContentBlock.Text)
		}
		if claudeResponse.ContentBlock.Type == "tool_use" {
			tools = append(tools, dto.ToolCallResponse{
				Index: platformruntime.GetPointer(fcIdx),
				ID:    claudeResponse.ContentBlock.Id,
				Type:  "function",
				Function: dto.FunctionResponse{
					Name:      claudeResponse.ContentBlock.Name,
					Arguments: "",
				},
			})
		}
	case "content_block_delta":
		if claudeResponse.Delta == nil {
			return nil
		}
		choice.Delta.Content = claudeResponse.Delta.Text
		switch claudeResponse.Delta.Type {
		case "input_json_delta":
			partialJSON := ""
			if claudeResponse.Delta.PartialJson != nil {
				partialJSON = *claudeResponse.Delta.PartialJson
			}
			tools = append(tools, dto.ToolCallResponse{
				Type:  "function",
				Index: platformruntime.GetPointer(fcIdx),
				Function: dto.FunctionResponse{
					Arguments: partialJSON,
				},
			})
		case "signature_delta":
			signatureContent := "\n"
			choice.Delta.ReasoningContent = &signatureContent
		case "thinking_delta":
			choice.Delta.ReasoningContent = claudeResponse.Delta.Thinking
		}
	case "message_delta":
		if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
			finishReason := stopReasonClaude2OpenAI(*claudeResponse.Delta.StopReason)
			if finishReason != "null" {
				choice.FinishReason = &finishReason
			}
		}
	case "message_stop":
		return nil
	default:
		return nil
	}
	if len(tools) > 0 {
		choice.Delta.Content = nil
		choice.Delta.ToolCalls = tools
	}
	response.Choices = append(response.Choices, choice)
	return &response
}

func ResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.OpenAITextResponse {
	fullTextResponse := dto.OpenAITextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", platformruntime.GetUUID()),
		Object:  "chat.completion",
		Created: platformruntime.GetTimestamp(),
	}
	var responseText string
	var responseThinking string
	if len(claudeResponse.Content) > 0 {
		responseText = claudeResponse.Content[0].GetText()
		if claudeResponse.Content[0].Thinking != nil {
			responseThinking = *claudeResponse.Content[0].Thinking
		}
	}
	tools := make([]dto.ToolCallResponse, 0)
	thinkingContent := ""
	fullTextResponse.Id = claudeResponse.Id
	for _, message := range claudeResponse.Content {
		switch message.Type {
		case "tool_use":
			args, _ := json.Marshal(message.Input)
			tools = append(tools, dto.ToolCallResponse{
				ID:   message.Id,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      message.Name,
					Arguments: string(args),
				},
			})
		case "thinking":
			if message.Thinking != nil {
				thinkingContent = *message.Thinking
			}
		case "text":
			responseText = message.GetText()
		}
	}
	choice := dto.OpenAITextResponseChoice{
		Index: 0,
		Message: dto.Message{
			Role: "assistant",
		},
		FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
	}
	choice.SetStringContent(responseText)
	if len(responseThinking) > 0 {
		choice.ReasoningContent = &responseThinking
	}
	if len(tools) > 0 {
		choice.Message.SetToolCalls(tools)
	}
	if thinkingContent != "" {
		choice.Message.ReasoningContent = &thinkingContent
	}
	fullTextResponse.Model = claudeResponse.Model
	fullTextResponse.Choices = []dto.OpenAITextResponseChoice{choice}
	return &fullTextResponse
}

type ClaudeResponseInfo struct {
	ResponseID   string
	Created      int64
	Model        string
	ResponseText strings.Builder
	Usage        *dto.Usage
	Done         bool
}

func cacheCreationTokensForOpenAIUsage(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	splitCacheCreationTokens := usage.ClaudeCacheCreation5mTokens + usage.ClaudeCacheCreation1hTokens
	if splitCacheCreationTokens == 0 {
		return usage.PromptTokensDetails.CachedCreationTokens
	}
	if usage.PromptTokensDetails.CachedCreationTokens > splitCacheCreationTokens {
		return usage.PromptTokensDetails.CachedCreationTokens
	}
	return splitCacheCreationTokens
}

func buildOpenAIStyleUsageFromClaudeUsage(usage *dto.Usage) dto.Usage {
	if usage == nil {
		return dto.Usage{}
	}
	clone := *usage
	clone.ClaudeCacheCreation5mTokens, clone.ClaudeCacheCreation1hTokens = tokenx.NormalizeCacheCreationSplit(
		usage.PromptTokensDetails.CachedCreationTokens,
		usage.ClaudeCacheCreation5mTokens,
		usage.ClaudeCacheCreation1hTokens,
	)
	cacheCreationTokens := cacheCreationTokensForOpenAIUsage(usage)
	totalInputTokens := usage.PromptTokens + usage.PromptTokensDetails.CachedTokens + cacheCreationTokens
	clone.PromptTokens = totalInputTokens
	clone.InputTokens = totalInputTokens
	clone.TotalTokens = totalInputTokens + usage.CompletionTokens
	clone.UsageSemantic = "openai"
	clone.UsageSource = "anthropic"
	return clone
}

func buildMessageDeltaPatchUsage(claudeResponse *dto.ClaudeResponse, claudeInfo *ClaudeResponseInfo) *dto.ClaudeUsage {
	usage := &dto.ClaudeUsage{}
	if claudeResponse != nil && claudeResponse.Usage != nil {
		*usage = *claudeResponse.Usage
	}
	if claudeInfo == nil || claudeInfo.Usage == nil {
		return usage
	}
	if usage.InputTokens == 0 && claudeInfo.Usage.PromptTokens > 0 {
		usage.InputTokens = claudeInfo.Usage.PromptTokens
	}
	if usage.CacheReadInputTokens == 0 && claudeInfo.Usage.PromptTokensDetails.CachedTokens > 0 {
		usage.CacheReadInputTokens = claudeInfo.Usage.PromptTokensDetails.CachedTokens
	}
	if usage.CacheCreationInputTokens == 0 && claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens > 0 {
		usage.CacheCreationInputTokens = claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens
	}
	cacheCreation5m, cacheCreation1h := claudeInfo.Usage.ClaudeCacheCreation5mTokens, claudeInfo.Usage.ClaudeCacheCreation1hTokens
	if usage.CacheCreation != nil {
		cacheCreation5m = usage.CacheCreation.Ephemeral5mInputTokens
		cacheCreation1h = usage.CacheCreation.Ephemeral1hInputTokens
	}
	cacheCreation5m, cacheCreation1h = tokenx.NormalizeCacheCreationSplit(
		usage.CacheCreationInputTokens,
		cacheCreation5m,
		cacheCreation1h,
	)
	if usage.CacheCreation == nil && (cacheCreation5m > 0 || cacheCreation1h > 0) {
		usage.CacheCreation = &dto.ClaudeCacheCreationUsage{}
	}
	if usage.CacheCreation != nil {
		usage.CacheCreation.Ephemeral5mInputTokens = cacheCreation5m
		usage.CacheCreation.Ephemeral1hInputTokens = cacheCreation1h
	}
	return usage
}

func shouldSkipClaudeMessageDeltaUsagePatch(info *relaycommon.RelayInfo) bool {
	if gatewaystore.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	return info != nil && info.ChannelSetting.PassThroughBodyEnabled
}

func patchClaudeMessageDeltaUsageData(data string, usage *dto.ClaudeUsage) string {
	if data == "" || usage == nil {
		return data
	}
	data = setMessageDeltaUsageInt(data, "usage.input_tokens", usage.InputTokens)
	data = setMessageDeltaUsageInt(data, "usage.cache_read_input_tokens", usage.CacheReadInputTokens)
	data = setMessageDeltaUsageInt(data, "usage.cache_creation_input_tokens", usage.CacheCreationInputTokens)
	if usage.CacheCreation != nil {
		data = setMessageDeltaUsageInt(data, "usage.cache_creation.ephemeral_5m_input_tokens", usage.CacheCreation.Ephemeral5mInputTokens)
		data = setMessageDeltaUsageInt(data, "usage.cache_creation.ephemeral_1h_input_tokens", usage.CacheCreation.Ephemeral1hInputTokens)
	}
	return data
}

func setMessageDeltaUsageInt(data string, path string, localValue int) string {
	if localValue <= 0 {
		return data
	}
	upstreamValue := gjson.Get(data, path)
	if upstreamValue.Exists() && upstreamValue.Int() > 0 {
		return data
	}
	patchedData, err := sjson.Set(data, path, localValue)
	if err != nil {
		return data
	}
	return patchedData
}

func FormatClaudeResponseInfo(claudeResponse *dto.ClaudeResponse, oaiResponse *dto.ChatCompletionsStreamResponse, claudeInfo *ClaudeResponseInfo) bool {
	if claudeInfo == nil {
		return false
	}
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &dto.Usage{}
	}
	switch claudeResponse.Type {
	case "message_start":
		if claudeResponse.Message != nil {
			claudeInfo.ResponseID = claudeResponse.Message.Id
			claudeInfo.Model = claudeResponse.Message.Model
			if claudeResponse.Message.Usage != nil {
				claudeInfo.Usage.PromptTokens = claudeResponse.Message.Usage.InputTokens
				claudeInfo.Usage.UsageSemantic = "anthropic"
				claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Message.Usage.CacheReadInputTokens
				claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Message.Usage.CacheCreationInputTokens
				claudeInfo.Usage.ClaudeCacheCreation5mTokens = claudeResponse.Message.Usage.GetCacheCreation5mTokens()
				claudeInfo.Usage.ClaudeCacheCreation1hTokens = claudeResponse.Message.Usage.GetCacheCreation1hTokens()
				claudeInfo.Usage.CompletionTokens = claudeResponse.Message.Usage.OutputTokens
			}
		}
	case "content_block_delta":
		if claudeResponse.Delta != nil {
			if claudeResponse.Delta.Text != nil {
				claudeInfo.ResponseText.WriteString(*claudeResponse.Delta.Text)
			}
			if claudeResponse.Delta.Thinking != nil {
				claudeInfo.ResponseText.WriteString(*claudeResponse.Delta.Thinking)
			}
		}
	case "message_delta":
		if claudeResponse.Usage != nil {
			claudeInfo.Usage.UsageSemantic = "anthropic"
			if claudeResponse.Usage.InputTokens > 0 {
				claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
			}
			if claudeResponse.Usage.CacheReadInputTokens > 0 {
				claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Usage.CacheReadInputTokens
			}
			if claudeResponse.Usage.CacheCreationInputTokens > 0 {
				claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Usage.CacheCreationInputTokens
			}
			if cacheCreation5m := claudeResponse.Usage.GetCacheCreation5mTokens(); cacheCreation5m > 0 {
				claudeInfo.Usage.ClaudeCacheCreation5mTokens = cacheCreation5m
			}
			if cacheCreation1h := claudeResponse.Usage.GetCacheCreation1hTokens(); cacheCreation1h > 0 {
				claudeInfo.Usage.ClaudeCacheCreation1hTokens = cacheCreation1h
			}
			if claudeResponse.Usage.OutputTokens > 0 {
				claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
			}
			claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens
		}
		claudeInfo.Done = true
	default:
		if claudeResponse.Type != "content_block_start" {
			return false
		}
	}
	if oaiResponse != nil {
		oaiResponse.Id = claudeInfo.ResponseID
		oaiResponse.Created = claudeInfo.Created
		oaiResponse.Model = claudeInfo.Model
	}
	return true
}

func HandleStreamResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, data string) *types.APIError {
	var claudeResponse dto.ClaudeResponse
	if err := platformencoding.UnmarshalString(data, &claudeResponse); err != nil {
		platformobservability.SysLog("error unmarshalling stream response: " + err.Error())
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if claudeResponse.StopReason != "" {
		maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	}
	if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
		maybeMarkClaudeRefusal(c, *claudeResponse.Delta.StopReason)
	}

	if info.RelayFormat == types.RelayFormatClaude {
		FormatClaudeResponseInfo(&claudeResponse, nil, claudeInfo)
		if claudeResponse.Type == "message_start" && claudeResponse.Message != nil {
			info.UpstreamModelName = claudeResponse.Message.Model
		} else if claudeResponse.Type == "message_delta" && !shouldSkipClaudeMessageDeltaUsagePatch(info) {
			data = patchClaudeMessageDeltaUsageData(data, buildMessageDeltaPatchUsage(&claudeResponse, claudeInfo))
		}
		if err := gatewaystream.ClaudeChunkData(c, claudeResponse, data); err != nil {
			logger.LogError(c, "send_stream_response_failed: "+err.Error())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(strings.ToLower(err.Error()), "context canceled") ||
				strings.Contains(strings.ToLower(err.Error()), "request context done") ||
				strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
				return nil
			}
			return types.NewError(err, types.ErrorCodeDoRequestFailed)
		}
		return nil
	}

	response := StreamResponseClaude2OpenAI(&claudeResponse)
	if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
		return nil
	}
	if err := gatewaystream.ObjectData(c, response); err != nil {
		logger.LogError(c, "send_stream_response_failed: "+err.Error())
	}
	return nil
}

func HandleStreamFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) {
	info.ConversationResponseText = claudeInfo.ResponseText.String()
	if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
		fallback := tokenx.ResponseText2Usage(c, claudeInfo.ResponseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		if claudeInfo.Usage.CompletionTokens == 0 || (!claudeInfo.Done && fallback.CompletionTokens > claudeInfo.Usage.CompletionTokens) {
			claudeInfo.Usage.CompletionTokens = fallback.CompletionTokens
		}
		if claudeInfo.Usage.PromptTokens == 0 {
			claudeInfo.Usage.PromptTokens = fallback.PromptTokens
		}
		claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens
	}
	if claudeInfo.Usage != nil {
		claudeInfo.Usage.UsageSemantic = "anthropic"
	}
	if info.RelayFormat == types.RelayFormatOpenAI {
		if info.ShouldIncludeUsage {
			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
			response := gatewaystream.GenerateFinalUsageResponse(claudeInfo.ResponseID, claudeInfo.Created, info.UpstreamModelName, openAIUsage)
			if err := gatewaystream.ObjectData(c, response); err != nil {
				platformobservability.SysLog("send final response failed: " + err.Error())
			}
		}
		gatewaystream.Done(c)
	}
}

func ClaudeStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.APIError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseID:   gatewaystream.GetResponseID(c),
		Created:      platformruntime.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var streamErr *types.APIError
	gatewaystream.ScanResponse(c, resp, info, func(data string, sr *gatewaystream.Result) {
		streamErr = HandleStreamResponseData(c, info, claudeInfo, data)
		if streamErr != nil {
			sr.Stop(streamErr)
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}
	HandleStreamFinalResponse(c, info, claudeInfo)
	return claudeInfo.Usage, nil
}

func HandleClaudeResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, httpResp *http.Response, data []byte) *types.APIError {
	var claudeResponse dto.ClaudeResponse
	if err := platformencoding.Unmarshal(data, &claudeResponse); err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &dto.Usage{}
	}
	if claudeResponse.Usage != nil {
		claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
		claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.TotalTokens = claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.UsageSemantic = "anthropic"
		claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Usage.CacheReadInputTokens
		claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Usage.CacheCreationInputTokens
		claudeInfo.Usage.ClaudeCacheCreation5mTokens = claudeResponse.Usage.GetCacheCreation5mTokens()
		claudeInfo.Usage.ClaudeCacheCreation1hTokens = claudeResponse.Usage.GetCacheCreation1hTokens()
	}

	var responseData []byte
	var err error
	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		openaiResponse.Usage = buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
		responseData, err = json.Marshal(openaiResponse)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatClaude:
		responseData = data
	}
	if claudeResponse.Usage != nil && claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}
	info.ConversationResponseText = claudeResponsePlainText(&claudeResponse)
	platformhttpx.IOCopyBytesGracefully(c, httpResp, responseData)
	return nil
}

func claudeResponsePlainText(response *dto.ClaudeResponse) string {
	if response == nil {
		return ""
	}
	parts := make([]string, 0, len(response.Content))
	for _, content := range response.Content {
		switch content.Type {
		case "text":
			parts = append(parts, content.GetText())
		case "thinking":
			if content.Thinking != nil {
				parts = append(parts, *content.Thinking)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func ClaudeHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.APIError) {
	defer platformhttpx.CloseResponseBodyGracefully(resp)
	claudeInfo := &ClaudeResponseInfo{
		ResponseID:   gatewaystream.GetResponseID(c),
		Created:      platformruntime.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if platformconfig.DebugEnabled {
		println("responseBody: ", string(responseBody))
	}
	handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody)
	if handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}

func mapToolChoice(toolChoice any, parallelToolCalls *bool) *dto.ClaudeToolChoice {
	var claudeToolChoice *dto.ClaudeToolChoice
	if toolChoiceStr, ok := toolChoice.(string); ok {
		switch toolChoiceStr {
		case "auto":
			claudeToolChoice = &dto.ClaudeToolChoice{Type: "auto"}
		case "required":
			claudeToolChoice = &dto.ClaudeToolChoice{Type: "any"}
		case "none":
			claudeToolChoice = &dto.ClaudeToolChoice{Type: "none"}
		}
	} else if toolChoiceMap, ok := toolChoice.(map[string]interface{}); ok {
		if function, ok := toolChoiceMap["function"].(map[string]interface{}); ok {
			if toolName, ok := function["name"].(string); ok {
				claudeToolChoice = &dto.ClaudeToolChoice{Type: "tool", Name: toolName}
			}
		}
	}
	if parallelToolCalls != nil {
		if claudeToolChoice == nil {
			claudeToolChoice = &dto.ClaudeToolChoice{Type: "auto"}
		}
		if claudeToolChoice.Type != "none" {
			claudeToolChoice.DisableParallelToolUse = !*parallelToolCalls
		}
	}
	return claudeToolChoice
}
