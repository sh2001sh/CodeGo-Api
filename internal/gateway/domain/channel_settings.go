package domain

import (
	"encoding/json"
	"fmt"

	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

func ValidateSettings(channel *gatewayschema.Channel) error {
	if channel == nil || channel.Setting == nil || *channel.Setting == "" {
		return nil
	}
	channelParams := &dto.ChannelSettings{}
	return platformencoding.Unmarshal([]byte(*channel.Setting), channelParams)
}

func GetSettings(channel *gatewayschema.Channel) dto.ChannelSettings {
	setting := dto.ChannelSettings{}
	if channel == nil || channel.Setting == nil || *channel.Setting == "" {
		return setting
	}
	if err := platformencoding.Unmarshal([]byte(*channel.Setting), &setting); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to unmarshal setting: channel_id=%d, error=%v", channel.Id, err))
	}
	return setting
}

func SetSettings(channel *gatewayschema.Channel, setting dto.ChannelSettings) {
	if channel == nil {
		return
	}
	settingBytes, err := platformencoding.Marshal(setting)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to marshal setting: channel_id=%d, error=%v", channel.Id, err))
		return
	}
	channel.Setting = platformruntime.GetPointer[string](string(settingBytes))
}

func GetOtherSettings(channel *gatewayschema.Channel) dto.ChannelOtherSettings {
	setting := dto.ChannelOtherSettings{}
	if channel == nil || channel.OtherSettings == "" {
		return setting
	}
	if err := platformencoding.UnmarshalString(channel.OtherSettings, &setting); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to unmarshal setting: channel_id=%d, error=%v", channel.Id, err))
	}
	return setting
}

func SetOtherSettings(channel *gatewayschema.Channel, setting dto.ChannelOtherSettings) {
	if channel == nil {
		return
	}
	settingBytes, err := platformencoding.Marshal(setting)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to marshal setting: channel_id=%d, error=%v", channel.Id, err))
		return
	}
	channel.OtherSettings = string(settingBytes)
}

func GetParamOverride(channel *gatewayschema.Channel) map[string]interface{} {
	paramOverride := make(map[string]interface{})
	if channel == nil || channel.ParamOverride == nil || *channel.ParamOverride == "" {
		return paramOverride
	}
	if err := platformencoding.Unmarshal([]byte(*channel.ParamOverride), &paramOverride); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to unmarshal param override: channel_id=%d, error=%v", channel.Id, err))
	}
	return paramOverride
}

func GetHeaderOverride(channel *gatewayschema.Channel) map[string]interface{} {
	headerOverride := make(map[string]interface{})
	if channel == nil || channel.HeaderOverride == nil || *channel.HeaderOverride == "" {
		return headerOverride
	}
	if err := platformencoding.Unmarshal([]byte(*channel.HeaderOverride), &headerOverride); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to unmarshal header override: channel_id=%d, error=%v", channel.Id, err))
	}
	return headerOverride
}

func GetOtherInfo(channel *gatewayschema.Channel) map[string]interface{} {
	otherInfo := make(map[string]interface{})
	if channel == nil || channel.OtherInfo == "" {
		return otherInfo
	}
	if err := platformencoding.Unmarshal([]byte(channel.OtherInfo), &otherInfo); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to unmarshal other info: channel_id=%d, tag=%s, name=%s, error=%v", channel.Id, channel.GetTag(), channel.Name, err))
	}
	return otherInfo
}

func SetOtherInfo(channel *gatewayschema.Channel, otherInfo map[string]interface{}) {
	if channel == nil {
		return
	}
	otherInfoBytes, err := json.Marshal(otherInfo)
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to marshal other info: channel_id=%d, tag=%s, name=%s, error=%v", channel.Id, channel.GetTag(), channel.Name, err))
		return
	}
	channel.OtherInfo = string(otherInfoBytes)
}
