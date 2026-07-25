package bootstrap

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"strconv"

	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayroutingapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/routing/app"
)

func startControlBackgroundTasks() {
	startOptionSyncLoop()

	if frequencyText := getenvTrimmed("CHANNEL_UPDATE_FREQUENCY"); frequencyText != "" {
		frequency, err := strconv.Atoi(frequencyText)
		if err != nil {
			platformobservability.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
			return
		}
		gatewayexecutionapp.StartChannelBalanceUpdateTask(frequency)
	}

	gatewayexecutionapp.StartAutomaticChannelTestTask()
	gatewayexecutionapp.StartCodexCredentialAutoRefreshTask()
	gatewayroutingapp.StartChannelUpstreamModelUpdateTask()
}
