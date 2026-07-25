package app

import (
	"errors"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/types"
	"time"
)

var ErrMultiKeyChannelBalanceUnsupported = errors.New("多密钥渠道不支持余额查询")

// UpdateChannelBalance refreshes one channel balance by id.
func UpdateChannelBalance(channelID int) (float64, error) {
	channel, err := gatewaystore.GetCachedChannel(channelID)
	if err != nil {
		return 0, err
	}
	if channel.ChannelInfo.IsMultiKey {
		return 0, ErrMultiKeyChannelBalanceUnsupported
	}
	return refreshChannelBalance(channel)
}

// UpdateAllChannelsBalance refreshes all enabled single-key channel balances.
func UpdateAllChannelsBalance() error {
	channels, err := gatewaystore.ListAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		if channel.Status != constant.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue
		}

		balance, updateErr := refreshChannelBalance(channel)
		if updateErr == nil && balance <= 0 {
			DisableChannel(
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()),
				"余额不足",
			)
		}

		time.Sleep(platformconfig.RequestInterval)
	}
	return nil
}
