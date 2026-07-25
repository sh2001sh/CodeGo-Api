package store

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

func LoadModelSupportedEndpointTypes(modelName string) []constant.EndpointType {
	return gatewaystore.LoadModelSupportedEndpointTypes(modelName)
}
