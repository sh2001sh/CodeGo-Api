package stream

import (
	"net/http"

	"github.com/gin-gonic/gin"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
)

type StreamResult = Result

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {
	ScanResponse(c, resp, info, dataHandler)
}
