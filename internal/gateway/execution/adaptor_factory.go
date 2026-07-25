package execution

import (
	providers "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
)

func NewSyncAdaptor(apiType int) providers.SyncAdaptor {
	return providers.NewSyncAdaptor(apiType)
}
