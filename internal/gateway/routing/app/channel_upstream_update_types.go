package app

import (
	"sync"
	"sync/atomic"
)

const (
	ChannelUpstreamModelUpdateTaskDefaultIntervalMinutes  = 30
	ChannelUpstreamModelUpdateTaskBatchSize               = 100
	ChannelUpstreamModelUpdateMinCheckIntervalSeconds     = 300
	ChannelUpstreamModelUpdateNotifySuppressWindowSeconds = 86400
	ChannelUpstreamModelUpdateNotifyMaxChannelDetails     = 8
	ChannelUpstreamModelUpdateNotifyMaxModelDetails       = 12
	ChannelUpstreamModelUpdateNotifyMaxFailedChannelIDs   = 10
)

var ChannelUpstreamModelUpdateSelectFields = []string{
	"id",
	"name",
	"type",
	"key",
	"status",
	"base_url",
	"models",
	"model_mapping",
	"settings",
	"setting",
	"other",
	"group",
	"priority",
	"weight",
	"tag",
	"channel_info",
	"header_override",
}

var (
	channelUpstreamModelUpdateTaskOnce    sync.Once
	channelUpstreamModelUpdateTaskRunning atomic.Bool
	channelUpstreamModelUpdateNotifyState = struct {
		sync.Mutex
		lastNotifiedAt      int64
		lastChangedChannels int
		lastFailedChannels  int
	}{}
)

type ApplyChannelUpstreamModelUpdatesRequest struct {
	ID           int      `json:"id"`
	AddModels    []string `json:"add_models"`
	RemoveModels []string `json:"remove_models"`
	IgnoreModels []string `json:"ignore_models"`
}

type ApplyAllChannelUpstreamModelUpdatesResult struct {
	ChannelID             int      `json:"channel_id"`
	ChannelName           string   `json:"channel_name"`
	AddedModels           []string `json:"added_models"`
	RemovedModels         []string `json:"removed_models"`
	RemainingModels       []string `json:"remaining_models"`
	RemainingRemoveModels []string `json:"remaining_remove_models"`
}

type DetectChannelUpstreamModelUpdatesResult struct {
	ChannelID       int      `json:"channel_id"`
	ChannelName     string   `json:"channel_name"`
	AddModels       []string `json:"add_models"`
	RemoveModels    []string `json:"remove_models"`
	LastCheckTime   int64    `json:"last_check_time"`
	AutoAddedModels int      `json:"auto_added_models"`
}

type upstreamModelUpdateChannelSummary struct {
	ChannelName string
	AddCount    int
	RemoveCount int
}

type ApplyChannelUpstreamModelUpdatesResult struct {
	ChannelID             int
	ChannelName           string
	AddedModels           []string
	RemovedModels         []string
	IgnoredModels         []string
	RemainingModels       []string
	RemainingRemoveModels []string
	Models                string
	Settings              string
}

type ApplyAllChannelUpstreamModelUpdatesSummary struct {
	ProcessedChannels int
	AddedModels       int
	RemovedModels     int
	FailedChannelIDs  []int
	Results           []ApplyAllChannelUpstreamModelUpdatesResult
}

type DetectAllChannelUpstreamModelUpdatesSummary struct {
	ProcessedChannels      int
	FailedChannelIDs       []int
	DetectedAddModels      int
	DetectedRemoveModels   int
	ChannelDetectedResults []DetectChannelUpstreamModelUpdatesResult
}
