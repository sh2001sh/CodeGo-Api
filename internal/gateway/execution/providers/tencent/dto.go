package tencent

type TencentMessage struct {
	Role    string `json:"Role"`
	Content string `json:"Content"`
}

type TencentChatRequest struct {
	Model       *string           `json:"Model"`
	Messages    []*TencentMessage `json:"Messages"`
	Stream      *bool             `json:"Stream,omitempty"`
	TopP        *float64          `json:"TopP,omitempty"`
	Temperature *float64          `json:"Temperature,omitempty"`
}

type TencentError struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
}

type TencentUsage struct {
	PromptTokens     int `json:"PromptTokens"`
	CompletionTokens int `json:"CompletionTokens"`
	TotalTokens      int `json:"TotalTokens"`
}

type TencentResponseChoices struct {
	FinishReason string         `json:"FinishReason,omitempty"`
	Messages     TencentMessage `json:"Message,omitempty"`
	Delta        TencentMessage `json:"Delta,omitempty"`
}

type TencentChatResponse struct {
	Choices []TencentResponseChoices `json:"Choices,omitempty"`
	Created int64                    `json:"Created,omitempty"`
	Id      string                   `json:"Id,omitempty"`
	Usage   TencentUsage             `json:"Usage,omitempty"`
	Error   TencentError             `json:"Error,omitempty"`
	Note    string                   `json:"Note,omitempty"`
	ReqID   string                   `json:"Req_id,omitempty"`
}

type TencentChatResponseSB struct {
	Response TencentChatResponse `json:"Response,omitempty"`
}
