package app

import (
	"encoding/json"
	"errors"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"strings"
	// ChannelTagUpdateRequest describes batch updates for channels under one tag.
)

type ChannelTagUpdateRequest struct {
	Tag            string  `json:"tag"`
	NewTag         *string `json:"new_tag"`
	Priority       *int64  `json:"priority"`
	Weight         *uint   `json:"weight"`
	ModelMapping   *string `json:"model_mapping"`
	Models         *string `json:"models"`
	Groups         *string `json:"groups"`
	ParamOverride  *string `json:"param_override"`
	HeaderOverride *string `json:"header_override"`
}

// ChannelBatchRequest describes batch mutations over channel IDs.
type ChannelBatchRequest struct {
	IDs []int   `json:"ids"`
	Tag *string `json:"tag"`
}

func validateTag(tag string) error {
	if strings.TrimSpace(tag) == "" {
		return errors.New("tag不能为空")
	}
	return nil
}

func normalizeJSONOverride(value *string, invalidMessage string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed != "" && !json.Valid([]byte(trimmed)) {
		return nil, errors.New(invalidMessage)
	}
	return platformruntime.GetPointer(trimmed), nil
}

func refreshChannelCache() {
	gatewaystore.InitChannelCache()
}

// DeleteDisabledChannels removes all disabled channels and refreshes cache.
func DeleteDisabledChannels() (int64, error) {
	rows, err := gatewaystore.DeleteDisabledChannels()
	if err != nil {
		return 0, err
	}
	refreshChannelCache()
	return rows, nil
}

// DisableChannelsByTag disables all channels under the given tag.
func DisableChannelsByTag(tag string) error {
	if err := validateTag(tag); err != nil {
		return err
	}
	if err := gatewaystore.DisableChannelsByTag(tag); err != nil {
		return err
	}
	refreshChannelCache()
	return nil
}

// EnableChannelsByTag enables all channels under the given tag.
func EnableChannelsByTag(tag string) error {
	if err := validateTag(tag); err != nil {
		return err
	}
	if err := gatewaystore.EnableChannelsByTag(tag); err != nil {
		return err
	}
	refreshChannelCache()
	return nil
}

// EditChannelsByTag updates shared fields for all channels under the given tag.
func EditChannelsByTag(req ChannelTagUpdateRequest) error {
	if err := validateTag(req.Tag); err != nil {
		return err
	}

	paramOverride, err := normalizeJSONOverride(req.ParamOverride, "参数覆盖必须是合法的 JSON 格式")
	if err != nil {
		return err
	}
	headerOverride, err := normalizeJSONOverride(req.HeaderOverride, "请求头覆盖必须是合法的 JSON 格式")
	if err != nil {
		return err
	}

	if err := gatewaystore.EditChannelsByTag(
		req.Tag,
		req.NewTag,
		req.ModelMapping,
		req.Models,
		req.Groups,
		req.Priority,
		req.Weight,
		paramOverride,
		headerOverride,
	); err != nil {
		return err
	}
	refreshChannelCache()
	return nil
}

// DeleteChannelsBatch deletes channels in one request and refreshes cache.
func DeleteChannelsBatch(ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("参数错误")
	}
	if err := gatewaystore.BatchDeleteChannels(ids); err != nil {
		return 0, err
	}
	refreshChannelCache()
	return len(ids), nil
}

// BatchSetChannelsTag updates one tag for multiple channels.
func BatchSetChannelsTag(ids []int, tag *string) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("参数错误")
	}
	if err := gatewaystore.BatchSetChannelTag(ids, tag); err != nil {
		return 0, err
	}
	refreshChannelCache()
	return len(ids), nil
}

// GetLongestTagModels returns the longest models list string for the tag.
func GetLongestTagModels(tag string) (string, error) {
	if err := validateTag(tag); err != nil {
		return "", err
	}

	channels, err := gatewaystore.ListChannelsByTag(tag, false, false)
	if err != nil {
		return "", err
	}

	longestModels := ""
	maxLength := 0
	for _, channel := range channels {
		if channel.Models == "" {
			continue
		}
		currentModels := strings.Split(channel.Models, ",")
		if len(currentModels) > maxLength {
			maxLength = len(currentModels)
			longestModels = channel.Models
		}
	}

	return longestModels, nil
}

// CopyChannel clones one channel and returns the new persisted record.
func CopyChannel(id int, suffix string, resetBalance bool) (*gatewayschema.Channel, error) {
	origin, err := gatewaystore.LoadChannelByID(id, true)
	if err != nil {
		return nil, err
	}

	clone := *origin
	clone.Id = 0
	clone.CreatedTime = platformruntime.GetTimestamp()
	clone.Name = origin.Name + suffix
	clone.TestTime = 0
	clone.ResponseTime = 0
	if resetBalance {
		clone.Balance = 0
		clone.UsedQuota = 0
	}

	if err := gatewaystore.CreateChannel(&clone); err != nil {
		return nil, err
	}
	refreshChannelCache()
	return &clone, nil
}
