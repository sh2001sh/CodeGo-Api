package translation

import (
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

func generateStopBlock(index int) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_stop",
		Index: platformruntime.GetPointer[int](index),
	}
}

func stopReasonOpenAI2Claude(reason string) string {
	return openAIFinishReasonToClaudeStopReason(reason)
}

func StreamResponseOpenAI2Claude(openAIResponse *dto.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) []*dto.ClaudeResponse {
	if info.ClaudeConvertInfo.Done {
		return nil
	}

	var claudeResponses []*dto.ClaudeResponse
	stopOpenBlocks := func() {
		switch info.ClaudeConvertInfo.LastMessagesType {
		case relaycommon.LastMessageTypeText, relaycommon.LastMessageTypeThinking:
			claudeResponses = append(claudeResponses, generateStopBlock(info.ClaudeConvertInfo.Index))
		case relaycommon.LastMessageTypeTools:
			base := info.ClaudeConvertInfo.ToolCallBaseIndex
			for offset := 0; offset <= info.ClaudeConvertInfo.ToolCallMaxIndexOffset; offset++ {
				claudeResponses = append(claudeResponses, generateStopBlock(base+offset))
			}
		}
	}
	stopOpenBlocksAndAdvance := func() {
		if info.ClaudeConvertInfo.LastMessagesType == relaycommon.LastMessageTypeNone {
			return
		}
		stopOpenBlocks()
		switch info.ClaudeConvertInfo.LastMessagesType {
		case relaycommon.LastMessageTypeTools:
			info.ClaudeConvertInfo.Index = info.ClaudeConvertInfo.ToolCallBaseIndex + info.ClaudeConvertInfo.ToolCallMaxIndexOffset + 1
			info.ClaudeConvertInfo.ToolCallBaseIndex = 0
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
		default:
			info.ClaudeConvertInfo.Index++
		}
		info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeNone
	}
	if info.SendResponseCount == 1 {
		msg := &dto.ClaudeMediaMessage{
			Id:    openAIResponse.Id,
			Model: openAIResponse.Model,
			Type:  "message",
			Role:  "assistant",
			Usage: &dto.ClaudeUsage{
				InputTokens:  info.GetEstimatePromptTokens(),
				OutputTokens: 0,
			},
		}
		msg.SetContent(make([]any, 0))
		claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
			Type:    "message_start",
			Message: msg,
		})
		if openAIResponse.IsToolCall() {
			info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeTools
			info.ClaudeConvertInfo.ToolCallBaseIndex = 0
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
			var toolCall dto.ToolCallResponse
			if len(openAIResponse.Choices) > 0 && len(openAIResponse.Choices[0].Delta.ToolCalls) > 0 {
				toolCall = openAIResponse.Choices[0].Delta.ToolCalls[0]
			} else {
				first := openAIResponse.GetFirstToolCall()
				if first != nil {
					toolCall = *first
				} else {
					toolCall = dto.ToolCallResponse{}
				}
			}
			resp := &dto.ClaudeResponse{
				Type: "content_block_start",
				ContentBlock: &dto.ClaudeMediaMessage{
					Id:    toolCall.ID,
					Type:  "tool_use",
					Name:  toolCall.Function.Name,
					Input: map[string]interface{}{},
				},
			}
			resp.SetIndex(0)
			claudeResponses = append(claudeResponses, resp)
			if toolCall.Function.Arguments != "" {
				idx := 0
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_delta",
					Delta: &dto.ClaudeMediaMessage{
						Type:        "input_json_delta",
						PartialJson: &toolCall.Function.Arguments,
					},
				})
			}
		}
		if len(openAIResponse.Choices) > 0 {
			reasoning := openAIResponse.Choices[0].Delta.GetReasoningContent()
			content := openAIResponse.Choices[0].Delta.GetContentString()

			if reasoning != "" {
				if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeThinking {
					stopOpenBlocksAndAdvance()
				}
				idx := info.ClaudeConvertInfo.Index
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_start",
					ContentBlock: &dto.ClaudeMediaMessage{
						Type:     "thinking",
						Thinking: platformruntime.GetPointer[string](""),
					},
				})
				idx2 := idx
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx2,
					Type:  "content_block_delta",
					Delta: &dto.ClaudeMediaMessage{
						Type:     "thinking_delta",
						Thinking: &reasoning,
					},
				})
				info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeThinking
			} else if content != "" {
				if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeText {
					stopOpenBlocksAndAdvance()
				}
				idx := info.ClaudeConvertInfo.Index
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_start",
					ContentBlock: &dto.ClaudeMediaMessage{
						Type: "text",
						Text: platformruntime.GetPointer[string](""),
					},
				})
				idx2 := idx
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx2,
					Type:  "content_block_delta",
					Delta: &dto.ClaudeMediaMessage{
						Type: "text_delta",
						Text: platformruntime.GetPointer[string](content),
					},
				})
				info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeText
			}
		}

		if len(openAIResponse.Choices) > 0 && openAIResponse.Choices[0].FinishReason != nil && *openAIResponse.Choices[0].FinishReason != "" {
			info.FinishReason = *openAIResponse.Choices[0].FinishReason
			stopOpenBlocks()
			oaiUsage := openAIResponse.Usage
			if oaiUsage == nil {
				oaiUsage = info.ClaudeConvertInfo.Usage
			}
			if oaiUsage != nil {
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Type:  "message_delta",
					Usage: buildClaudeUsageFromOpenAIUsage(oaiUsage),
					Delta: &dto.ClaudeMediaMessage{
						StopReason: platformruntime.GetPointer[string](stopReasonOpenAI2Claude(info.FinishReason)),
					},
				})
			}
			claudeResponses = append(claudeResponses, &dto.ClaudeResponse{Type: "message_stop"})
			info.ClaudeConvertInfo.Done = true
		}
		return claudeResponses
	}

	if len(openAIResponse.Choices) == 0 {
		oaiUsage := openAIResponse.Usage
		if oaiUsage == nil {
			oaiUsage = info.ClaudeConvertInfo.Usage
		}
		if oaiUsage != nil {
			stopOpenBlocks()
			stopReason := stopReasonOpenAI2Claude(info.FinishReason)
			if stopReason == "" {
				stopReason = "end_turn"
			}
			claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
				Type:  "message_delta",
				Usage: buildClaudeUsageFromOpenAIUsage(oaiUsage),
				Delta: &dto.ClaudeMediaMessage{
					StopReason: platformruntime.GetPointer[string](stopReason),
				},
			})
			claudeResponses = append(claudeResponses, &dto.ClaudeResponse{Type: "message_stop"})
			info.ClaudeConvertInfo.Done = true
		}
		return claudeResponses
	}

	chosenChoice := openAIResponse.Choices[0]
	doneChunk := chosenChoice.FinishReason != nil && *chosenChoice.FinishReason != ""
	if doneChunk {
		info.FinishReason = *chosenChoice.FinishReason
		oaiUsage := openAIResponse.Usage
		if oaiUsage == nil {
			oaiUsage = info.ClaudeConvertInfo.Usage
			return claudeResponses
		}
	}

	var claudeResponse dto.ClaudeResponse
	var isEmpty bool
	claudeResponse.Type = "content_block_delta"
	if len(chosenChoice.Delta.ToolCalls) > 0 {
		toolCalls := chosenChoice.Delta.ToolCalls
		if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeTools {
			stopOpenBlocksAndAdvance()
			info.ClaudeConvertInfo.ToolCallBaseIndex = info.ClaudeConvertInfo.Index
			info.ClaudeConvertInfo.ToolCallMaxIndexOffset = 0
		}
		info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeTools
		base := info.ClaudeConvertInfo.ToolCallBaseIndex
		maxOffset := info.ClaudeConvertInfo.ToolCallMaxIndexOffset

		for i, toolCall := range toolCalls {
			offset := 0
			if toolCall.Index != nil {
				offset = *toolCall.Index
			} else {
				offset = i
			}
			if offset > maxOffset {
				maxOffset = offset
			}
			blockIndex := base + offset

			idx := blockIndex
			if toolCall.Function.Name != "" {
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_start",
					ContentBlock: &dto.ClaudeMediaMessage{
						Id:    toolCall.ID,
						Type:  "tool_use",
						Name:  toolCall.Function.Name,
						Input: map[string]interface{}{},
					},
				})
			}

			if len(toolCall.Function.Arguments) > 0 {
				claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
					Index: &idx,
					Type:  "content_block_delta",
					Delta: &dto.ClaudeMediaMessage{
						Type:        "input_json_delta",
						PartialJson: &toolCall.Function.Arguments,
					},
				})
			}
		}
		info.ClaudeConvertInfo.ToolCallMaxIndexOffset = maxOffset
		info.ClaudeConvertInfo.Index = base + maxOffset
	} else {
		reasoning := chosenChoice.Delta.GetReasoningContent()
		textContent := chosenChoice.Delta.GetContentString()
		if reasoning != "" || textContent != "" {
			if reasoning != "" {
				if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeThinking {
					stopOpenBlocksAndAdvance()
					idx := info.ClaudeConvertInfo.Index
					claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &dto.ClaudeMediaMessage{
							Type:     "thinking",
							Thinking: platformruntime.GetPointer[string](""),
						},
					})
				}
				info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeThinking
				claudeResponse.Delta = &dto.ClaudeMediaMessage{
					Type:     "thinking_delta",
					Thinking: &reasoning,
				}
			} else {
				if info.ClaudeConvertInfo.LastMessagesType != relaycommon.LastMessageTypeText {
					stopOpenBlocksAndAdvance()
					idx := info.ClaudeConvertInfo.Index
					claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
						Index: &idx,
						Type:  "content_block_start",
						ContentBlock: &dto.ClaudeMediaMessage{
							Type: "text",
							Text: platformruntime.GetPointer[string](""),
						},
					})
				}
				info.ClaudeConvertInfo.LastMessagesType = relaycommon.LastMessageTypeText
				claudeResponse.Delta = &dto.ClaudeMediaMessage{
					Type: "text_delta",
					Text: platformruntime.GetPointer[string](textContent),
				}
			}
		} else {
			isEmpty = true
		}
	}

	claudeResponse.Index = platformruntime.GetPointer[int](info.ClaudeConvertInfo.Index)
	if !isEmpty && claudeResponse.Delta != nil {
		claudeResponses = append(claudeResponses, &claudeResponse)
	}

	if doneChunk || info.ClaudeConvertInfo.Done {
		stopOpenBlocks()
		oaiUsage := openAIResponse.Usage
		if oaiUsage == nil {
			oaiUsage = info.ClaudeConvertInfo.Usage
		}
		if oaiUsage != nil {
			claudeResponses = append(claudeResponses, &dto.ClaudeResponse{
				Type:  "message_delta",
				Usage: buildClaudeUsageFromOpenAIUsage(oaiUsage),
				Delta: &dto.ClaudeMediaMessage{
					StopReason: platformruntime.GetPointer[string](stopReasonOpenAI2Claude(info.FinishReason)),
				},
			})
		}
		claudeResponses = append(claudeResponses, &dto.ClaudeResponse{Type: "message_stop"})
		info.ClaudeConvertInfo.Done = true
		return claudeResponses
	}

	return claudeResponses
}
