package zhipu_4v

import (
	"strings"

	"github.com/sh2001sh/CodeGo-Api/dto"
)

func requestOpenAI2Zhipu(request dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	messages := make([]dto.Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		if !message.IsStringContent() {
			mediaMessages := message.ParseContent()
			for j, mediaMessage := range mediaMessages {
				if mediaMessage.Type == dto.ContentTypeImageURL {
					imageURL := mediaMessage.GetImageMedia()
					if strings.HasPrefix(imageURL.Url, "data:image/") {
						if idx := strings.Index(imageURL.Url, ","); idx != -1 {
							imageURL.Url = imageURL.Url[idx+1:]
						}
					}
					mediaMessage.ImageUrl = imageURL
					mediaMessages[j] = mediaMessage
				}
			}
			message.SetMediaContent(mediaMessages)
		}
		messages = append(messages, dto.Message{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallId: message.ToolCallId,
		})
	}
	str, ok := request.Stop.(string)
	var stop []string
	if ok {
		stop = []string{str}
	} else {
		stop, _ = request.Stop.([]string)
	}
	out := &dto.GeneralOpenAIRequest{
		Model:       request.Model,
		Stream:      request.Stream,
		Messages:    messages,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stop:        stop,
		Tools:       request.Tools,
		ToolChoice:  request.ToolChoice,
		THINKING:    request.THINKING,
	}
	if request.MaxTokens != nil || request.MaxCompletionTokens != nil {
		maxTokens := request.GetMaxTokens()
		out.MaxTokens = &maxTokens
	}
	return out
}
