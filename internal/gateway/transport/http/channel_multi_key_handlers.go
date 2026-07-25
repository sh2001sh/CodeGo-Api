package http

import (
	"errors"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func handleMultiKeyBusinessError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gatewayroutingapp.ErrChannelNotFound) ||
		errors.Is(err, gatewayroutingapp.ErrNotMultiKeyChannel) ||
		errors.Is(err, gatewayroutingapp.ErrMissingKeyIndex) ||
		errors.Is(err, gatewayroutingapp.ErrKeyIndexOutOfRange) ||
		errors.Is(err, gatewayroutingapp.ErrCannotDeleteLastKey) ||
		errors.Is(err, gatewayroutingapp.ErrNoKeysToDisable) ||
		errors.Is(err, gatewayroutingapp.ErrNoAutoDisabledKeys) ||
		errors.Is(err, gatewayroutingapp.ErrUnsupportedMultiKeyOp) {
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return true
	}
	return false
}

func respondMultiKeyMutation(c *gin.Context, result *gatewayroutingapp.MultiKeyOperationResult, err error) {
	if handleMultiKeyBusinessError(c, err) {
		return
	}
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	payload := gin.H{
		"success": true,
		"message": result.Message,
	}
	if result.Data != nil {
		payload["data"] = result.Data
	}
	c.JSON(stdhttp.StatusOK, payload)
}

func ManageMultiKeys(c *gin.Context) {
	var req gatewayroutingapp.MultiKeyManageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidParams(c)
		return
	}

	switch req.Action {
	case "get_key_status":
		result, err := gatewayroutingapp.GetMultiKeyStatus(req.ChannelID, req.Page, req.PageSize, req.Status)
		if handleMultiKeyBusinessError(c, err) {
			return
		}
		if err != nil {
			httpapi.ApiError(c, err)
			return
		}
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    result,
		})
	case "disable_key":
		result, err := gatewayroutingapp.DisableMultiKey(req.ChannelID, req.KeyIndex)
		respondMultiKeyMutation(c, result, err)
	case "enable_key":
		result, err := gatewayroutingapp.EnableMultiKey(req.ChannelID, req.KeyIndex)
		respondMultiKeyMutation(c, result, err)
	case "enable_all_keys":
		result, err := gatewayroutingapp.EnableAllMultiKeys(req.ChannelID)
		respondMultiKeyMutation(c, result, err)
	case "disable_all_keys":
		result, err := gatewayroutingapp.DisableAllMultiKeys(req.ChannelID)
		respondMultiKeyMutation(c, result, err)
	case "delete_key":
		result, err := gatewayroutingapp.DeleteMultiKey(req.ChannelID, req.KeyIndex)
		respondMultiKeyMutation(c, result, err)
	case "delete_disabled_keys":
		result, err := gatewayroutingapp.DeleteAutoDisabledMultiKeys(req.ChannelID)
		respondMultiKeyMutation(c, result, err)
	default:
		c.JSON(stdhttp.StatusOK, gin.H{
			"success": false,
			"message": gatewayroutingapp.ErrUnsupportedMultiKeyOp.Error(),
		})
	}
}
