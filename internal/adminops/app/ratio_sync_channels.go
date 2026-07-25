package app

import (
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

// ListSyncableChannels returns channels and built-in presets that support ratio sync.
func ListSyncableChannels() ([]dto.SyncableChannel, error) {
	channels, err := gatewaystore.ListAllChannels(0, 0, true, false)
	if err != nil {
		return nil, err
	}

	syncableChannels := make([]dto.SyncableChannel, 0, len(channels)+2)
	for _, channel := range channels {
		if channel.GetBaseURL() == "" {
			continue
		}
		syncableChannels = append(syncableChannels, dto.SyncableChannel{
			ID:      channel.Id,
			Name:    channel.Name,
			BaseURL: channel.GetBaseURL(),
			Status:  channel.Status,
			Type:    channel.Type,
		})
	}
	syncableChannels = append(syncableChannels, dto.SyncableChannel{
		ID:      officialRatioPresetID,
		Name:    officialRatioPresetName,
		BaseURL: officialRatioPresetBaseURL,
		Status:  1,
	})
	syncableChannels = append(syncableChannels, dto.SyncableChannel{
		ID:      modelsDevPresetID,
		Name:    modelsDevPresetName,
		BaseURL: modelsDevPresetBaseURL,
		Status:  1,
	})
	return syncableChannels, nil
}
