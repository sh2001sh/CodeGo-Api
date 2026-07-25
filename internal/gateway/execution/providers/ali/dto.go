package ali

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformfilex "github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
)

type AliMessage struct {
	Content any    `json:"content"`
	Role    string `json:"role"`
}

type AliMediaContent struct {
	Image string `json:"image,omitempty"`
	Text  string `json:"text,omitempty"`
}

type AliInput struct {
	Prompt   string       `json:"prompt,omitempty"`
	Messages []AliMessage `json:"messages"`
}

type AliParameters struct {
	TopP              float64 `json:"top_p,omitempty"`
	TopK              int     `json:"top_k,omitempty"`
	Seed              uint64  `json:"seed,omitempty"`
	EnableSearch      bool    `json:"enable_search,omitempty"`
	IncrementalOutput bool    `json:"incremental_output,omitempty"`
}

type AliChatRequest struct {
	Model      string        `json:"model"`
	Input      AliInput      `json:"input,omitempty"`
	Parameters AliParameters `json:"parameters,omitempty"`
}

type AliEmbeddingRequest struct {
	Model string `json:"model"`
	Input struct {
		Texts []string `json:"texts"`
	} `json:"input"`
	Parameters *struct {
		TextType string `json:"text_type,omitempty"`
	} `json:"parameters,omitempty"`
}

type AliEmbedding struct {
	Embedding []float64 `json:"embedding"`
	TextIndex int       `json:"text_index"`
}

type AliEmbeddingResponse struct {
	Output struct {
		Embeddings []AliEmbedding `json:"embeddings"`
	} `json:"output"`
	Usage AliUsage `json:"usage"`
	AliError
}

type AliError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
}

type AliUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	ImageCount   int `json:"image_count,omitempty"`
}

type TaskResult struct {
	B64Image string `json:"b64_image,omitempty"`
	Url      string `json:"url,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

type AliOutput struct {
	TaskId       string       `json:"task_id,omitempty"`
	TaskStatus   string       `json:"task_status,omitempty"`
	Text         string       `json:"text"`
	FinishReason string       `json:"finish_reason"`
	Message      string       `json:"message,omitempty"`
	Code         string       `json:"code,omitempty"`
	Results      []TaskResult `json:"results,omitempty"`
	Choices      []struct {
		FinishReason string `json:"finish_reason,omitempty"`
		Message      struct {
			Role             string            `json:"role,omitempty"`
			Content          []AliMediaContent `json:"content,omitempty"`
			ReasoningContent string            `json:"reasoning_content,omitempty"`
		} `json:"message,omitempty"`
	} `json:"choices,omitempty"`
}

func (o *AliOutput) ChoicesToOpenAIImageDate(c *gin.Context, responseFormat string) []dto.ImageData {
	var imageData []dto.ImageData
	if len(o.Choices) > 0 {
		for _, choice := range o.Choices {
			var data dto.ImageData
			for _, content := range choice.Message.Content {
				if content.Image != "" {
					if strings.HasPrefix(content.Image, "http") {
						var b64JSON string
						if responseFormat == "b64_json" {
							_, b64, err := platformfilex.GetImageFromURL(content.Image)
							if err != nil {
								logger.LogError(c, "get_image_data_failed: "+err.Error())
								continue
							}
							b64JSON = b64
						}
						data.Url = content.Image
						data.B64Json = b64JSON
					} else {
						data.B64Json = content.Image
					}
				} else if content.Text != "" {
					data.RevisedPrompt = content.Text
				}
			}
			imageData = append(imageData, data)
		}
	}

	return imageData
}

func (o *AliOutput) ResultToOpenAIImageDate(c *gin.Context, responseFormat string) []dto.ImageData {
	var imageData []dto.ImageData
	for _, data := range o.Results {
		var b64JSON string
		if responseFormat == "b64_json" {
			_, b64, err := platformfilex.GetImageFromURL(data.Url)
			if err != nil {
				logger.LogError(c, "get_image_data_failed: "+err.Error())
				continue
			}
			b64JSON = b64
		} else {
			b64JSON = data.B64Image
		}

		imageData = append(imageData, dto.ImageData{
			Url:           data.Url,
			B64Json:       b64JSON,
			RevisedPrompt: "",
		})
	}
	return imageData
}

type AliResponse struct {
	Output AliOutput `json:"output"`
	Usage  AliUsage  `json:"usage"`
	AliError
}

type AliImageRequest struct {
	Model          string             `json:"model"`
	Input          any                `json:"input"`
	Parameters     AliImageParameters `json:"parameters,omitempty"`
	ResponseFormat string             `json:"response_format,omitempty"`
}

type AliImageParameters struct {
	Size             string `json:"size,omitempty"`
	N                int    `json:"n,omitempty"`
	Steps            string `json:"steps,omitempty"`
	Scale            string `json:"scale,omitempty"`
	Watermark        *bool  `json:"watermark,omitempty"`
	PromptExtend     *bool  `json:"prompt_extend,omitempty"`
	ThinkingMode     *bool  `json:"thinking_mode,omitempty"`
	EnableSequential *bool  `json:"enable_sequential,omitempty"`
	BboxList         any    `json:"bbox_list,omitempty"`
	ColorPalette     any    `json:"color_palette,omitempty"`
	Seed             *int   `json:"seed,omitempty"`
}

func (p *AliImageParameters) PromptExtendValue() bool {
	if p != nil && p.PromptExtend != nil {
		return *p.PromptExtend
	}
	return false
}

type AliImageInput struct {
	Prompt         string       `json:"prompt,omitempty"`
	NegativePrompt string       `json:"negative_prompt,omitempty"`
	Messages       []AliMessage `json:"messages,omitempty"`
}

type WanImageInput struct {
	Prompt         string   `json:"prompt"`
	Images         []string `json:"images"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
}

type WanImageParameters struct {
	N         int     `json:"n,omitempty"`
	Watermark *bool   `json:"watermark,omitempty"`
	Seed      int     `json:"seed,omitempty"`
	Strength  float64 `json:"strength,omitempty"`
}

type AliRerankParameters struct {
	TopN            *int  `json:"top_n,omitempty"`
	ReturnDocuments *bool `json:"return_documents,omitempty"`
}

type AliRerankInput struct {
	Query     string `json:"query"`
	Documents []any  `json:"documents"`
}

type AliRerankRequest struct {
	Model      string              `json:"model"`
	Input      AliRerankInput      `json:"input"`
	Parameters AliRerankParameters `json:"parameters,omitempty"`
}

type AliRerankResponse struct {
	Output struct {
		Results []dto.RerankResponseResult `json:"results"`
	} `json:"output"`
	Usage     AliUsage `json:"usage"`
	RequestId string   `json:"request_id"`
	AliError
}
