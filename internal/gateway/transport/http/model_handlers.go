package http

import (
	"fmt"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func ListModels(c *gin.Context, modelType int) {
	userID := c.GetInt("id")
	tokenModelLimitEnabled := httpctx.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
	tokenModelLimit := map[string]bool{}
	if tokenModelLimitEnabled {
		if value, ok := httpctx.GetContextKey(c, constant.ContextKeyTokenModelLimit); ok {
			tokenModelLimit = value.(map[string]bool)
		}
	}
	tokenGroup := httpctx.GetContextKeyString(c, constant.ContextKeyTokenGroup)

	userOpenAIModels, err := gatewayroutingapp.CollectUserOpenAIModels(userID, tokenModelLimitEnabled, tokenModelLimit, tokenGroup)
	if err != nil {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": "get user group failed",
		})
		return
	}
	switch modelType {
	case constant.ChannelTypeAnthropic:
		anthropicModels := gatewayroutingapp.BuildAnthropicModels(userOpenAIModels)
		firstID := ""
		lastID := ""
		if len(anthropicModels) > 0 {
			firstID = anthropicModels[0].ID
			lastID = anthropicModels[len(anthropicModels)-1].ID
		}
		c.JSON(stdhttp.StatusOK, gin.H{
			"data":     anthropicModels,
			"first_id": firstID,
			"has_more": false,
			"last_id":  lastID,
		})
	case constant.ChannelTypeGemini:
		c.JSON(stdhttp.StatusOK, gin.H{
			"models":        gatewayroutingapp.BuildGeminiModels(userOpenAIModels),
			"nextPageToken": nil,
		})
	default:
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": true,
			"data":    userOpenAIModels,
			"object":  "list",
		})
	}
}

func ChannelListModels(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    gatewayroutingapp.AllChannelModels(),
	})
}

func DashboardListModels(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    gatewayroutingapp.DashboardModels(),
	})
}

func EnabledListModels(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"data":    gatewayroutingapp.EnabledModels(),
	})
}

func RetrieveModel(c *gin.Context, modelType int) {
	modelID := c.Param("model")
	if aiModel, ok := gatewayroutingapp.FindOpenAIModel(modelID); ok {
		switch modelType {
		case constant.ChannelTypeAnthropic:
			c.JSON(stdhttp.StatusOK, gatewayroutingapp.BuildAnthropicModels([]dto.OpenAIModels{aiModel})[0])
		default:
			c.JSON(stdhttp.StatusOK, aiModel)
		}
		return
	}

	openAIError := types.OpenAIError{
		Message: fmt.Sprintf("The model '%s' does not exist", modelID),
		Type:    "invalid_request_error",
		Param:   "model",
		Code:    "model_not_found",
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"error": openAIError,
	})
}
