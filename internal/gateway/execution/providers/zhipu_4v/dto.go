package zhipu_4v

import (
	"time"

	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type ZhipuV4Response struct {
	Id                  string                         `json:"id"`
	Created             int64                          `json:"created"`
	Model               string                         `json:"model"`
	TextResponseChoices []dto.OpenAITextResponseChoice `json:"choices"`
	Usage               dto.Usage                      `json:"usage"`
	Error               types.OpenAIError              `json:"error"`
}

type ZhipuV4StreamResponse struct {
	Id      string                                    `json:"id"`
	Created int64                                     `json:"created"`
	Choices []dto.ChatCompletionsStreamResponseChoice `json:"choices"`
	Usage   dto.Usage                                 `json:"usage"`
}

type tokenData struct {
	Token      string
	ExpiryTime time.Time
}
