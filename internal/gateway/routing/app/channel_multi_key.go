package app

import (
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"strings"
)

var (
	ErrChannelNotFound       = errors.New("渠道不存在")
	ErrNotMultiKeyChannel    = errors.New("该渠道不是多密钥模式")
	ErrMissingKeyIndex       = errors.New("未指定要操作的密钥索引")
	ErrKeyIndexOutOfRange    = errors.New("密钥索引超出范围")
	ErrCannotDeleteLastKey   = errors.New("不能删除最后一个密钥")
	ErrNoKeysToDisable       = errors.New("没有可禁用的密钥")
	ErrNoAutoDisabledKeys    = errors.New("没有需要删除的自动禁用密钥")
	ErrUnsupportedMultiKeyOp = errors.New("不支持的操作")
)

// MultiKeyManageRequest describes supported multi-key operations.
type MultiKeyManageRequest struct {
	ChannelID int    `json:"channel_id"`
	Action    string `json:"action"`
	KeyIndex  *int   `json:"key_index,omitempty"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	Status    *int   `json:"status,omitempty"`
}

// MultiKeyStatusResponse describes paginated multi-key status data.
type MultiKeyStatusResponse struct {
	Keys                []KeyStatus `json:"keys"`
	Total               int         `json:"total"`
	Page                int         `json:"page"`
	PageSize            int         `json:"page_size"`
	TotalPages          int         `json:"total_pages"`
	EnabledCount        int         `json:"enabled_count"`
	ManualDisabledCount int         `json:"manual_disabled_count"`
	AutoDisabledCount   int         `json:"auto_disabled_count"`
}

// KeyStatus represents one key's runtime state in a multi-key channel.
type KeyStatus struct {
	Index        int    `json:"index"`
	Status       int    `json:"status"`
	DisabledTime int64  `json:"disabled_time,omitempty"`
	Reason       string `json:"reason,omitempty"`
	KeyPreview   string `json:"key_preview"`
}

// MultiKeyOperationResult describes a mutating multi-key operation result.
type MultiKeyOperationResult struct {
	Message string
	Data    any
}

func getManagedMultiKeyChannel(channelID int) (*gatewayschema.Channel, error) {
	channel, err := gatewaystore.LoadChannelByID(channelID, true)
	if err != nil {
		return nil, ErrChannelNotFound
	}
	if !channel.ChannelInfo.IsMultiKey {
		return nil, ErrNotMultiKeyChannel
	}
	return channel, nil
}

func requireValidMultiKeyIndex(channel *gatewayschema.Channel, keyIndex *int) (int, error) {
	if keyIndex == nil {
		return 0, ErrMissingKeyIndex
	}
	index := *keyIndex
	if index < 0 || index >= channel.ChannelInfo.MultiKeySize {
		return 0, ErrKeyIndexOutOfRange
	}
	return index, nil
}

func ensureMultiKeyStatusMaps(channel *gatewayschema.Channel) {
	if channel.ChannelInfo.MultiKeyStatusList == nil {
		channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
	}
	if channel.ChannelInfo.MultiKeyDisabledTime == nil {
		channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
	}
	if channel.ChannelInfo.MultiKeyDisabledReason == nil {
		channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
	}
}

func saveManagedMultiKeyChannel(channel *gatewayschema.Channel) error {
	if err := gatewaystore.UpdateChannel(channel); err != nil {
		return err
	}
	refreshChannelRuntimeCache()
	return nil
}

func buildKeyPreview(key string) string {
	if len(key) <= 10 {
		return key
	}
	return key[:10] + "..."
}

func readMultiKeyStatus(channel *gatewayschema.Channel, index int) int {
	if channel.ChannelInfo.MultiKeyStatusList == nil {
		return constant.ChannelStatusEnabled
	}
	if status, ok := channel.ChannelInfo.MultiKeyStatusList[index]; ok {
		return status
	}
	return constant.ChannelStatusEnabled
}

func filterKeyStatuses(statuses []KeyStatus, statusFilter *int) []KeyStatus {
	if statusFilter == nil {
		return statuses
	}
	filtered := make([]KeyStatus, 0, len(statuses))
	for _, status := range statuses {
		if status.Status == *statusFilter {
			filtered = append(filtered, status)
		}
	}
	return filtered
}

func paginateKeyStatuses(statuses []KeyStatus, page int, pageSize int) ([]KeyStatus, int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	total := len(statuses)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	if start >= total {
		return []KeyStatus{}, page, totalPages
	}
	return statuses[start:end], page, totalPages
}

// GetMultiKeyStatus returns paginated key statuses for a multi-key channel.
func GetMultiKeyStatus(channelID int, page int, pageSize int, statusFilter *int) (*MultiKeyStatusResponse, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	keys := channel.GetKeys()
	allStatuses := make([]KeyStatus, 0, len(keys))
	var enabledCount, manualDisabledCount, autoDisabledCount int
	for index, key := range keys {
		status := readMultiKeyStatus(channel, index)
		switch status {
		case constant.ChannelStatusEnabled:
			enabledCount++
		case constant.ChannelStatusManuallyDisabled:
			manualDisabledCount++
		case constant.ChannelStatusAutoDisabled:
			autoDisabledCount++
		}

		keyStatus := KeyStatus{
			Index:      index,
			Status:     status,
			KeyPreview: buildKeyPreview(key),
		}
		if status != constant.ChannelStatusEnabled {
			if channel.ChannelInfo.MultiKeyDisabledTime != nil {
				keyStatus.DisabledTime = channel.ChannelInfo.MultiKeyDisabledTime[index]
			}
			if channel.ChannelInfo.MultiKeyDisabledReason != nil {
				keyStatus.Reason = channel.ChannelInfo.MultiKeyDisabledReason[index]
			}
		}
		allStatuses = append(allStatuses, keyStatus)
	}

	filteredStatuses := filterKeyStatuses(allStatuses, statusFilter)
	pageStatuses, currentPage, totalPages := paginateKeyStatuses(filteredStatuses, page, pageSize)

	return &MultiKeyStatusResponse{
		Keys:                pageStatuses,
		Total:               len(filteredStatuses),
		Page:                currentPage,
		PageSize:            pageSizeOrDefault(pageSize),
		TotalPages:          totalPages,
		EnabledCount:        enabledCount,
		ManualDisabledCount: manualDisabledCount,
		AutoDisabledCount:   autoDisabledCount,
	}, nil
}

func pageSizeOrDefault(pageSize int) int {
	if pageSize <= 0 {
		return 50
	}
	return pageSize
}

// DisableMultiKey manually disables one key.
func DisableMultiKey(channelID int, keyIndex *int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	index, err := requireValidMultiKeyIndex(channel, keyIndex)
	if err != nil {
		return nil, err
	}

	ensureMultiKeyStatusMaps(channel)
	channel.ChannelInfo.MultiKeyStatusList[index] = constant.ChannelStatusManuallyDisabled

	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{Message: "密钥已禁用"}, nil
}

// EnableMultiKey clears disabled state for one key.
func EnableMultiKey(channelID int, keyIndex *int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	index, err := requireValidMultiKeyIndex(channel, keyIndex)
	if err != nil {
		return nil, err
	}

	if channel.ChannelInfo.MultiKeyStatusList != nil {
		delete(channel.ChannelInfo.MultiKeyStatusList, index)
	}
	if channel.ChannelInfo.MultiKeyDisabledTime != nil {
		delete(channel.ChannelInfo.MultiKeyDisabledTime, index)
	}
	if channel.ChannelInfo.MultiKeyDisabledReason != nil {
		delete(channel.ChannelInfo.MultiKeyDisabledReason, index)
	}

	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{Message: "密钥已启用"}, nil
}

// EnableAllMultiKeys clears all disabled states.
func EnableAllMultiKeys(channelID int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	enabledCount := 0
	if channel.ChannelInfo.MultiKeyStatusList != nil {
		enabledCount = len(channel.ChannelInfo.MultiKeyStatusList)
	}

	channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
	channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
	channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)

	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{Message: fmt.Sprintf("已启用 %d 个密钥", enabledCount)}, nil
}

// DisableAllMultiKeys disables every currently enabled key.
func DisableAllMultiKeys(channelID int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	ensureMultiKeyStatusMaps(channel)
	disabledCount := 0
	for index := 0; index < channel.ChannelInfo.MultiKeySize; index++ {
		if readMultiKeyStatus(channel, index) != constant.ChannelStatusEnabled {
			continue
		}
		channel.ChannelInfo.MultiKeyStatusList[index] = constant.ChannelStatusManuallyDisabled
		disabledCount++
	}

	if disabledCount == 0 {
		return nil, ErrNoKeysToDisable
	}
	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{Message: fmt.Sprintf("已禁用 %d 个密钥", disabledCount)}, nil
}

// DeleteMultiKey deletes one key and reindexes its status maps.
func DeleteMultiKey(channelID int, keyIndex *int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	index, err := requireValidMultiKeyIndex(channel, keyIndex)
	if err != nil {
		return nil, err
	}

	keys := channel.GetKeys()
	if len(keys) == 1 {
		return nil, ErrCannotDeleteLastKey
	}

	remainingKeys := make([]string, 0, len(keys)-1)
	newStatusList := make(map[int]int)
	newDisabledTime := make(map[int]int64)
	newDisabledReason := make(map[int]string)

	nextIndex := 0
	for currentIndex, key := range keys {
		if currentIndex == index {
			continue
		}
		remainingKeys = append(remainingKeys, key)
		status := readMultiKeyStatus(channel, currentIndex)
		if status != constant.ChannelStatusEnabled {
			newStatusList[nextIndex] = status
		}
		if channel.ChannelInfo.MultiKeyDisabledTime != nil {
			if disabledTime, ok := channel.ChannelInfo.MultiKeyDisabledTime[currentIndex]; ok {
				newDisabledTime[nextIndex] = disabledTime
			}
		}
		if channel.ChannelInfo.MultiKeyDisabledReason != nil {
			if reason, ok := channel.ChannelInfo.MultiKeyDisabledReason[currentIndex]; ok {
				newDisabledReason[nextIndex] = reason
			}
		}
		nextIndex++
	}

	channel.Key = strings.Join(remainingKeys, "\n")
	channel.ChannelInfo.MultiKeySize = len(remainingKeys)
	channel.ChannelInfo.MultiKeyStatusList = newStatusList
	channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
	channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{Message: "密钥已删除"}, nil
}

// DeleteAutoDisabledMultiKeys deletes only auto-disabled keys.
func DeleteAutoDisabledMultiKeys(channelID int) (*MultiKeyOperationResult, error) {
	channel, err := getManagedMultiKeyChannel(channelID)
	if err != nil {
		return nil, err
	}

	lock := gatewaystore.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	keys := channel.GetKeys()
	remainingKeys := make([]string, 0, len(keys))
	newStatusList := make(map[int]int)
	newDisabledTime := make(map[int]int64)
	newDisabledReason := make(map[int]string)
	deletedCount := 0
	nextIndex := 0

	for currentIndex, key := range keys {
		status := readMultiKeyStatus(channel, currentIndex)
		if status == constant.ChannelStatusAutoDisabled {
			deletedCount++
			continue
		}

		remainingKeys = append(remainingKeys, key)
		if status != constant.ChannelStatusEnabled {
			newStatusList[nextIndex] = status
		}
		if channel.ChannelInfo.MultiKeyDisabledTime != nil {
			if disabledTime, ok := channel.ChannelInfo.MultiKeyDisabledTime[currentIndex]; ok {
				newDisabledTime[nextIndex] = disabledTime
			}
		}
		if channel.ChannelInfo.MultiKeyDisabledReason != nil {
			if reason, ok := channel.ChannelInfo.MultiKeyDisabledReason[currentIndex]; ok {
				newDisabledReason[nextIndex] = reason
			}
		}
		nextIndex++
	}

	if deletedCount == 0 {
		return nil, ErrNoAutoDisabledKeys
	}

	channel.Key = strings.Join(remainingKeys, "\n")
	channel.ChannelInfo.MultiKeySize = len(remainingKeys)
	channel.ChannelInfo.MultiKeyStatusList = newStatusList
	channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
	channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

	if err := saveManagedMultiKeyChannel(channel); err != nil {
		return nil, err
	}
	return &MultiKeyOperationResult{
		Message: fmt.Sprintf("已删除 %d 个自动禁用的密钥", deletedCount),
		Data:    deletedCount,
	}, nil
}
