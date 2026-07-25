package store

import (
	"fmt"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

func SaveChannelInfo(channel *gatewayschema.Channel) error {
	return platformdb.DB.Model(channel).Update("channel_info", channel.ChannelInfo).Error
}

func SaveChannelWithoutKey(channel *gatewayschema.Channel) error {
	if channel.Id == 0 {
		return fmt.Errorf("channel ID is 0")
	}
	return platformdb.DB.Omit("key").Save(channel).Error
}

func CreateChannel(channel *gatewayschema.Channel) error {
	if err := platformdb.DB.Create(channel).Error; err != nil {
		return err
	}
	return AddChannelAbilities(channel, nil)
}

func UpdateChannel(channel *gatewayschema.Channel) error {
	prepareChannelForSave(channel)

	if err := platformdb.DB.Model(channel).Updates(channel).Error; err != nil {
		return err
	}
	platformdb.DB.Model(channel).First(channel, "id = ?", channel.Id)
	return UpdateChannelAbilities(channel, nil)
}

func DeleteChannelByID(channelID int) error {
	channel := &gatewayschema.Channel{Id: channelID}
	if err := platformdb.DB.Delete(channel).Error; err != nil {
		return err
	}
	return DeleteChannelAbilities(channelID)
}

func UpdateChannelResponseTime(channel *gatewayschema.Channel, responseTime int64) {
	err := platformdb.DB.Model(channel).Select("response_time", "test_time").Updates(gatewayschema.Channel{
		TestTime:     platformruntime.GetTimestamp(),
		ResponseTime: int(responseTime),
	}).Error
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to update response time: channel_id=%d, error=%v", channel.Id, err))
	}
}

func UpdateChannelBalance(channel *gatewayschema.Channel, balance float64) {
	err := platformdb.DB.Model(channel).Select("balance_updated_time", "balance").Updates(gatewayschema.Channel{
		BalanceUpdatedTime: platformruntime.GetTimestamp(),
		Balance:            balance,
	}).Error
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to update balance: channel_id=%d, error=%v", channel.Id, err))
	}
}

func prepareChannelForSave(channel *gatewayschema.Channel) {
	if !channel.ChannelInfo.IsMultiKey {
		return
	}

	keyStr := channel.Key
	if keyStr == "" {
		if existing, err := LoadChannelByID(channel.Id, true); err == nil {
			keyStr = existing.Key
		}
	}

	keys := (&gatewayschema.Channel{Key: keyStr}).GetKeys()
	channel.ChannelInfo.MultiKeySize = len(keys)
	if channel.ChannelInfo.MultiKeyStatusList != nil {
		for index := range channel.ChannelInfo.MultiKeyStatusList {
			if index >= channel.ChannelInfo.MultiKeySize {
				delete(channel.ChannelInfo.MultiKeyStatusList, index)
			}
		}
	}
}
