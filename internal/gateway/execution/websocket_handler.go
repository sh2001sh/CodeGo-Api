package execution

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sh2001sh/CodeGo-Api/dto"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func WssHelper(c *gin.Context, info *relaycommon.RelayInfo) (apiError *types.APIError) {
	info.InitChannelMeta(c)

	adaptor := NewSyncAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, nil)
	if err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	if resp != nil {
		info.TargetWs = resp.(*websocket.Conn)
		defer info.TargetWs.Close()
	}

	usage, apiError := adaptor.DoResponse(c, nil, info)
	if apiError != nil {
		platformhttpx.ResetStatusCode(apiError, statusCodeMappingStr)
		return apiError
	}
	billingapp.PostWssConsumeQuota(c, info, info.UpstreamModelName, usage.(*dto.RealtimeUsage), "")
	return nil
}
