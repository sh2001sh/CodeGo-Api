package domain

import (
	"testing"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	"github.com/stretchr/testify/assert"
)

func TestChannelSettingsParsingDoesNotMutateSourceFields(t *testing.T) {
	rawSetting := "{bad json"
	channel := &gatewayschema.Channel{
		Id:            1,
		Setting:       &rawSetting,
		OtherSettings: "{bad json",
	}

	setting := GetSettings(channel)
	otherSettings := GetOtherSettings(channel)

	assert.Equal(t, rawSetting, *channel.Setting)
	assert.Equal(t, "{bad json", channel.OtherSettings)
	assert.Empty(t, setting.Proxy)
	assert.Empty(t, otherSettings.VertexKeyType)
}
