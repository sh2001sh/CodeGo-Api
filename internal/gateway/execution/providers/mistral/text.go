package mistral

import (
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"regexp"
)

var mistralToolCallIDRegexp = regexp.MustCompile("^[a-zA-Z0-9]{9}$")

func requestOpenAI2Mistral(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	messages := make([]dto.Message, 0, len(request.Messages))
	idMap := make(map[string]string)
	for _, message := range request.Messages {
		toolCalls := message.ParseToolCalls()
		if toolCalls != nil {
			for i := range toolCalls {
				if !mistralToolCallIDRegexp.MatchString(toolCalls[i].ID) {
					if newID, ok := idMap[toolCalls[i].ID]; ok {
						toolCalls[i].ID = newID
					} else {
						newID, err := platformruntime.GenerateRandomCharsKey(9)
						if err == nil {
							idMap[toolCalls[i].ID] = newID
							toolCalls[i].ID = newID
						}
					}
				}
			}
			message.SetToolCalls(toolCalls)
		}

		if message.ToolCallId != "" {
			if newID, ok := idMap[message.ToolCallId]; ok {
				message.ToolCallId = newID
			} else if !mistralToolCallIDRegexp.MatchString(message.ToolCallId) {
				newID, err := platformruntime.GenerateRandomCharsKey(9)
				if err == nil {
					idMap[message.ToolCallId] = newID
					message.ToolCallId = newID
				}
			}
		}

		mediaMessages := message.ParseContent()
		if message.Role == "assistant" && message.ToolCalls != nil && message.Content == "" {
			mediaMessages = []dto.MediaContent{}
		}
		for i, mediaMessage := range mediaMessages {
			if mediaMessage.Type == dto.ContentTypeImageURL {
				imageURL := mediaMessage.GetImageMedia()
				mediaMessage.ImageUrl = imageURL.Url
				mediaMessages[i] = mediaMessage
			}
		}
		message.SetMediaContent(mediaMessages)
		messages = append(messages, dto.Message{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallId: message.ToolCallId,
		})
	}

	out := &dto.GeneralOpenAIRequest{
		Model:       request.Model,
		Stream:      request.Stream,
		Messages:    messages,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Tools:       request.Tools,
		ToolChoice:  request.ToolChoice,
	}
	if request.MaxTokens != nil || request.MaxCompletionTokens != nil {
		maxTokens := request.GetMaxTokens()
		out.MaxTokens = &maxTokens
	}
	return out
}
