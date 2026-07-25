package app

import (
	"encoding/json"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"strings"
	// AddChannelRequest describes channel creation modes in the admin API.
)

type AddChannelRequest struct {
	Mode                      string                 `json:"mode"`
	MultiKeyMode              constant.MultiKeyMode  `json:"multi_key_mode"`
	BatchAddSetKeyPrefix2Name bool                   `json:"batch_add_set_key_prefix_2_name"`
	Channel                   *gatewayschema.Channel `json:"channel"`
}

// ChannelPatch describes an admin channel update payload.
type ChannelPatch struct {
	gatewayschema.Channel
	MultiKeyMode *string `json:"multi_key_mode"`
	KeyMode      *string `json:"key_mode"`
}

func validateChannel(channel *gatewayschema.Channel, isAdd bool) error {
	if channel == nil {
		return fmt.Errorf("channel cannot be empty")
	}
	if err := gatewaydomain.ValidateSettings(channel); err != nil {
		return fmt.Errorf("渠道额外设置[channel setting] 格式错误：%s", err.Error())
	}

	if isAdd && channel.Key == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if isAdd {
		for _, name := range channel.GetModels() {
			if len(name) > 255 {
				return fmt.Errorf("模型名称过长: %s", name)
			}
		}
	}

	if channel.Type == constant.ChannelTypeVertexAi {
		if channel.Other == "" {
			return fmt.Errorf("部署地区不能为空")
		}
		regionMap, err := platformtext.StrToMap(channel.Other)
		if err != nil {
			return fmt.Errorf("部署地区必须是标准的Json格式，例如{\"default\": \"us-central1\", \"region2\": \"us-east1\"}")
		}
		if regionMap["default"] == nil {
			return fmt.Errorf("部署地区必须包含default字段")
		}
	}

	if channel.Type == constant.ChannelTypeCodex {
		trimmedKey := strings.TrimSpace(channel.Key)
		if isAdd || trimmedKey != "" {
			if !strings.HasPrefix(trimmedKey, "{") {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			var keyMap map[string]any
			if err := platformencoding.Unmarshal([]byte(trimmedKey), &keyMap); err != nil {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			if value, ok := keyMap["access_token"]; !ok || value == nil || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
				return fmt.Errorf("Codex key JSON must include access_token")
			}
			if value, ok := keyMap["account_id"]; !ok || value == nil || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
				return fmt.Errorf("Codex key JSON must include account_id")
			}
		}
	}

	return nil
}

func getVertexArrayKeys(keys string) ([]string, error) {
	if keys == "" {
		return nil, nil
	}
	var keyArray []interface{}
	if err := platformencoding.Unmarshal([]byte(keys), &keyArray); err != nil {
		return nil, fmt.Errorf("批量添加 Vertex AI 必须使用标准的JsonArray格式，例如[{key1}, {key2}...]，请检查输入: %w", err)
	}

	cleanKeys := make([]string, 0, len(keyArray))
	for _, key := range keyArray {
		var keyStr string
		switch value := key.(type) {
		case string:
			keyStr = strings.TrimSpace(value)
		default:
			bytes, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("Vertex AI key JSON 编码失败: %w", err)
			}
			keyStr = string(bytes)
		}
		if keyStr != "" {
			cleanKeys = append(cleanKeys, keyStr)
		}
	}
	if len(cleanKeys) == 0 {
		return nil, fmt.Errorf("批量添加 Vertex AI 的 keys 不能为空")
	}
	return cleanKeys, nil
}

func normalizeLineKeys(keys string) []string {
	rawKeys := strings.Split(keys, "\n")
	cleanKeys := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		cleanKeys = append(cleanKeys, trimmed)
	}
	return cleanKeys
}

func buildAddKeys(req AddChannelRequest) ([]string, error) {
	switch req.Mode {
	case "multi_to_single":
		req.Channel.ChannelInfo.IsMultiKey = true
		req.Channel.ChannelInfo.MultiKeyMode = req.MultiKeyMode
		if req.Channel.Type == constant.ChannelTypeVertexAi && gatewaydomain.GetOtherSettings(req.Channel).VertexKeyType != dto.VertexKeyTypeAPIKey {
			keys, err := getVertexArrayKeys(req.Channel.Key)
			if err != nil {
				return nil, err
			}
			req.Channel.ChannelInfo.MultiKeySize = len(keys)
			req.Channel.Key = strings.Join(keys, "\n")
			return []string{req.Channel.Key}, nil
		}
		keys := normalizeLineKeys(req.Channel.Key)
		req.Channel.ChannelInfo.MultiKeySize = len(keys)
		req.Channel.Key = strings.Join(keys, "\n")
		return []string{req.Channel.Key}, nil
	case "batch":
		if req.Channel.Type == constant.ChannelTypeVertexAi && gatewaydomain.GetOtherSettings(req.Channel).VertexKeyType != dto.VertexKeyTypeAPIKey {
			return getVertexArrayKeys(req.Channel.Key)
		}
		return normalizeLineKeys(req.Channel.Key), nil
	case "single":
		return []string{req.Channel.Key}, nil
	default:
		return nil, fmt.Errorf("不支持的添加模式")
	}
}

func createChannelsForInsert(req AddChannelRequest, keys []string) []gatewayschema.Channel {
	baseChannel := *req.Channel
	channels := make([]gatewayschema.Channel, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		channel := baseChannel
		channel.Key = key
		if req.BatchAddSetKeyPrefix2Name && len(keys) > 1 {
			keyPrefix := channel.Key
			if len(channel.Key) > 8 {
				keyPrefix = channel.Key[:8]
			}
			channel.Name = fmt.Sprintf("%s %s", baseChannel.Name, keyPrefix)
		}
		channels = append(channels, channel)
	}
	return channels
}

// AddChannel creates channel records for the admin API.
func AddChannel(req AddChannelRequest) error {
	if err := validateChannel(req.Channel, true); err != nil {
		return err
	}

	req.Channel.CreatedTime = platformruntime.GetTimestamp()
	keys, err := buildAddKeys(req)
	if err != nil {
		return err
	}

	channels := createChannelsForInsert(req, keys)
	if err := gatewaystore.BatchInsertChannels(channels); err != nil {
		return err
	}
	refreshChannelRuntimeCache()
	return nil
}

func mergeAppendedKeys(origin *gatewayschema.Channel, patch *ChannelPatch) (string, error) {
	if origin.Key == "" {
		return patch.Key, nil
	}

	var existingKeys []string
	if strings.HasPrefix(strings.TrimSpace(origin.Key), "[") {
		var rawKeys []json.RawMessage
		if err := json.Unmarshal([]byte(strings.TrimSpace(origin.Key)), &rawKeys); err == nil {
			existingKeys = make([]string, len(rawKeys))
			for index, value := range rawKeys {
				existingKeys[index] = string(value)
			}
		}
	}
	if len(existingKeys) == 0 {
		existingKeys = strings.Split(strings.Trim(origin.Key, "\n"), "\n")
	}

	var newKeys []string
	if patch.Type == constant.ChannelTypeVertexAi && gatewaydomain.GetOtherSettings(&patch.Channel).VertexKeyType != dto.VertexKeyTypeAPIKey {
		if strings.HasPrefix(strings.TrimSpace(patch.Key), "[") {
			arrayKeys, err := getVertexArrayKeys(patch.Key)
			if err != nil {
				return "", fmt.Errorf("追加密钥解析失败: %s", err.Error())
			}
			newKeys = arrayKeys
		} else {
			newKeys = []string{patch.Key}
		}
	} else {
		newKeys = normalizeLineKeys(patch.Key)
	}

	seen := make(map[string]struct{}, len(existingKeys)+len(newKeys))
	for _, key := range existingKeys {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		seen[normalized] = struct{}{}
	}

	dedupedNewKeys := make([]string, 0, len(newKeys))
	for _, key := range newKeys {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		dedupedNewKeys = append(dedupedNewKeys, normalized)
	}

	return strings.Join(append(existingKeys, dedupedNewKeys...), "\n"), nil
}

// UpdateChannel updates one channel for the admin API.
func UpdateChannel(patch ChannelPatch) (*ChannelPatch, error) {
	if err := validateChannel(&patch.Channel, false); err != nil {
		return nil, err
	}

	originChannel, err := gatewaystore.LoadChannelByID(patch.Id, true)
	if err != nil {
		return nil, err
	}

	patch.ChannelInfo = originChannel.ChannelInfo
	if patch.MultiKeyMode != nil && *patch.MultiKeyMode != "" {
		patch.ChannelInfo.MultiKeyMode = constant.MultiKeyMode(*patch.MultiKeyMode)
	}

	if patch.KeyMode != nil && patch.ChannelInfo.IsMultiKey {
		switch *patch.KeyMode {
		case "append":
			mergedKey, err := mergeAppendedKeys(originChannel, &patch)
			if err != nil {
				return nil, err
			}
			patch.Key = mergedKey
		case "replace":
		}
	}

	if err := gatewaystore.UpdateChannel(&patch.Channel); err != nil {
		return nil, err
	}
	if originChannel.Status == constant.ChannelStatusEnabled && patch.Status != constant.ChannelStatusEnabled {
		gatewayruntime.InvalidateChannelAffinityForChannel(patch.Id)
	}
	refreshChannelRuntimeCache()
	patch.Key = ""
	sanitizeChannel(&patch.Channel)
	return &patch, nil
}

// DeleteChannel deletes one channel for the admin API.
func DeleteChannel(id int) error {
	if err := gatewaystore.DeleteChannelByID(id); err != nil {
		return err
	}
	gatewayruntime.InvalidateChannelAffinityForChannel(id)
	refreshChannelRuntimeCache()
	return nil
}
