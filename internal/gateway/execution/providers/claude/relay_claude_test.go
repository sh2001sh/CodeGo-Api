package claude

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/stretchr/testify/require"
)

func commonPointer[T any](value T) *T {
	return &value
}

func TestFormatClaudeResponseInfoMessageStart(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_start",
		Message: &dto.ClaudeMediaMessage{
			Id:    "msg_123",
			Model: "claude-3-5-sonnet",
			Usage: &dto.ClaudeUsage{
				InputTokens:              100,
				OutputTokens:             1,
				CacheCreationInputTokens: 50,
				CacheReadInputTokens:     30,
			},
		},
	}

	require.True(t, FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo))
	require.Equal(t, 100, claudeInfo.Usage.PromptTokens)
	require.Equal(t, 30, claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 50, claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, "msg_123", claudeInfo.ResponseID)
	require.Equal(t, "claude-3-5-sonnet", claudeInfo.Model)
}

func TestFormatClaudeResponseInfoMessageDeltaFullUsage(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens: 1,
		},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			InputTokens:              100,
			OutputTokens:             200,
			CacheCreationInputTokens: 50,
			CacheReadInputTokens:     30,
		},
	}

	require.True(t, FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo))
	require.Equal(t, 100, claudeInfo.Usage.PromptTokens)
	require.Equal(t, 200, claudeInfo.Usage.CompletionTokens)
	require.Equal(t, 300, claudeInfo.Usage.TotalTokens)
	require.True(t, claudeInfo.Done)
}

func TestFormatClaudeResponseInfoMessageDeltaOnlyOutputTokens(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
			CompletionTokens:            1,
			ClaudeCacheCreation5mTokens: 10,
			ClaudeCacheCreation1hTokens: 20,
		},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type:  "message_delta",
		Usage: &dto.ClaudeUsage{OutputTokens: 200},
	}

	require.True(t, FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo))
	require.Equal(t, 100, claudeInfo.Usage.PromptTokens)
	require.Equal(t, 200, claudeInfo.Usage.CompletionTokens)
	require.Equal(t, 300, claudeInfo.Usage.TotalTokens)
	require.Equal(t, 30, claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 50, claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, 10, claudeInfo.Usage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 20, claudeInfo.Usage.ClaudeCacheCreation1hTokens)
	require.True(t, claudeInfo.Done)
}

func TestFormatClaudeResponseInfoNilClaudeInfo(t *testing.T) {
	require.False(t, FormatClaudeResponseInfo(&dto.ClaudeResponse{Type: "message_start"}, nil, nil))
}

func TestFormatClaudeResponseInfoContentBlockDelta(t *testing.T) {
	text := "hello"
	claudeInfo := &ClaudeResponseInfo{
		Usage:        &dto.Usage{},
		ResponseText: strings.Builder{},
	}
	claudeResponse := &dto.ClaudeResponse{
		Type: "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Text: &text,
		},
	}

	require.True(t, FormatClaudeResponseInfo(claudeResponse, nil, claudeInfo))
	require.Equal(t, "hello", claudeInfo.ResponseText.String())
}

func TestBuildOpenAIStyleUsageFromClaudeUsage(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
		UsageSemantic:               "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)
	require.Equal(t, 180, openAIUsage.PromptTokens)
	require.Equal(t, 180, openAIUsage.InputTokens)
	require.Equal(t, 200, openAIUsage.TotalTokens)
	require.Equal(t, "openai", openAIUsage.UsageSemantic)
	require.Equal(t, "anthropic", openAIUsage.UsageSource)
}

func TestBuildOpenAIStyleUsageFromClaudeUsagePreservesCacheCreationRemainder(t *testing.T) {
	tests := []struct {
		name                    string
		cachedCreationTokens    int
		cacheCreationTokens5m   int
		cacheCreationTokens1h   int
		expectedTotalInputToken int
	}{
		{
			name:                    "prefers aggregate when it includes remainder",
			cachedCreationTokens:    50,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 180,
		},
		{
			name:                    "falls back to split tokens when aggregate missing",
			cachedCreationTokens:    0,
			cacheCreationTokens5m:   10,
			cacheCreationTokens1h:   20,
			expectedTotalInputToken: 160,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &dto.Usage{
				PromptTokens:     100,
				CompletionTokens: 20,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: tt.cachedCreationTokens,
				},
				ClaudeCacheCreation5mTokens: tt.cacheCreationTokens5m,
				ClaudeCacheCreation1hTokens: tt.cacheCreationTokens1h,
				UsageSemantic:               "anthropic",
			}

			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)
			require.Equal(t, tt.expectedTotalInputToken, openAIUsage.PromptTokens)
			require.Equal(t, tt.expectedTotalInputToken, openAIUsage.InputTokens)
		})
	}
}

func TestBuildOpenAIStyleUsageFromClaudeUsageDefaultsAggregateCacheCreationTo5m(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         30,
			CachedCreationTokens: 50,
		},
		UsageSemantic: "anthropic",
	}

	openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(usage)
	require.Equal(t, 50, openAIUsage.ClaudeCacheCreation5mTokens)
	require.Equal(t, 0, openAIUsage.ClaudeCacheCreation1hTokens)
}

func TestRequestOpenAI2ClaudeMessageIgnoresUnsupportedFileContent(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet",
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				dto.MediaContent{Type: dto.ContentTypeText, Text: "see attachment"},
				dto.MediaContent{
					Type: dto.ContentTypeFile,
					File: &dto.MessageFile{FileName: "blob.bin", FileData: "JVBERi0xLjQK"},
				},
			},
		}},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Len(t, claudeRequest.Messages, 1)

	content, ok := claudeRequest.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 1)
	require.Equal(t, "text", content[0].Type)
	require.NotNil(t, content[0].Text)
	require.Equal(t, "see attachment", *content[0].Text)
}

func TestRequestOpenAI2ClaudeMessageClaudeOpus48HighUsesAdaptiveThinking(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-high",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{{
			Role:    "user",
			Content: "hello",
		}},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}

func TestRequestOpenAI2ClaudeMessageClaudeOpus48ThinkingUsesAdaptiveHighEffort(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-8-thinking",
		Temperature: commonPointer(0.7),
		TopP:        commonPointer(0.9),
		TopK:        commonPointer(40),
		Messages: []dto.Message{{
			Role:    "user",
			Content: "hello",
		}},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)
}

func TestRequestOpenAI2ClaudeMessageSupportsPDFFileContent(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet",
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				dto.MediaContent{
					Type: dto.ContentTypeFile,
					File: &dto.MessageFile{FileName: "spec.pdf", FileData: "JVBERi0xLjQK"},
				},
				dto.MediaContent{Type: dto.ContentTypeText, Text: "summarize it"},
			},
		}},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Len(t, claudeRequest.Messages, 1)

	content, ok := claudeRequest.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 2)
	require.Equal(t, "document", content[0].Type)
	require.NotNil(t, content[0].Source)
	require.Equal(t, "base64", content[0].Source.Type)
	require.Equal(t, "application/pdf", content[0].Source.MediaType)
	require.Equal(t, "JVBERi0xLjQK", content[0].Source.Data)
	require.Equal(t, "text", content[1].Type)
	require.NotNil(t, content[1].Text)
	require.Equal(t, "summarize it", *content[1].Text)
}

func TestRequestOpenAI2ClaudeMessageConvertsTextFileContentToText(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet",
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				dto.MediaContent{
					Type: dto.ContentTypeFile,
					File: &dto.MessageFile{
						FileName: "notes.txt",
						FileData: base64.StdEncoding.EncodeToString([]byte("alpha\nbeta")),
					},
				},
			},
		}},
	}

	claudeRequest, err := RequestOpenAI2ClaudeMessage(nil, request)
	require.NoError(t, err)
	require.Len(t, claudeRequest.Messages, 1)

	content, ok := claudeRequest.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 1)
	require.Equal(t, "text", content[0].Type)
	require.NotNil(t, content[0].Text)
	require.Equal(t, "alpha\nbeta", *content[0].Text)
}
