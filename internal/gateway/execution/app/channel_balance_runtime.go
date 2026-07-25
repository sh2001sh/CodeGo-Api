package app

import (
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"time"
)

// StartChannelBalanceUpdateTask starts the periodic channel balance refresh task.
func StartChannelBalanceUpdateTask(frequency int) {
	go func() {
		for {
			time.Sleep(time.Duration(frequency) * time.Minute)
			platformobservability.SysLog(fmt.Sprintf("updating all channels: frequency=%d minutes", frequency))
			_ = UpdateAllChannelsBalance()
			platformobservability.SysLog("channels update done")
		}
	}()
}
