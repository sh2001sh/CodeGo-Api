package runtime

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func WssObject(c *gin.Context, ws *websocket.Conn, object interface{}) error {
	jsonData, err := platformencoding.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	return ws.WriteMessage(1, jsonData)
}

func WssError(c *gin.Context, ws *websocket.Conn, openAIError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openAIError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(constant.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}
