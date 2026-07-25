package app

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/types"
)

type channelTestResult struct {
	context  *gin.Context
	localErr error
	apiError *types.APIError
}
