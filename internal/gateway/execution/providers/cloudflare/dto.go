package cloudflare

import "github.com/sh2001sh/CodeGo-Api/dto"

type CFRequest struct {
	Messages    []dto.Message `json:"messages,omitempty"`
	Lora        string        `json:"lora,omitempty"`
	MaxTokens   uint          `json:"max_tokens,omitempty"`
	Prompt      string        `json:"prompt,omitempty"`
	Raw         bool          `json:"raw,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type CFAudioResponse struct {
	Result CFSTTResult `json:"result"`
}

type CFSTTResult struct {
	Text string `json:"text"`
}
