package store

import (
	"errors"
	"fmt"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"

	"github.com/sh2001sh/CodeGo-Api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

var group2model2channels map[string]map[string][]int
var channelsIDM map[int]*gatewayschema.Channel
var channelSyncLock sync.RWMutex
var channelStatusLock sync.Mutex
var channelPollingLocks sync.Map

// LoadChannelByID loads a channel by ID, optionally including sensitive fields.
func LoadChannelByID(channelID int, selectAll bool) (*gatewayschema.Channel, error) {
	channel := &gatewayschema.Channel{Id: channelID}
	var err error
	if selectAll {
		err = platformdb.DB.First(channel, "id = ?", channelID).Error
	} else {
		err = platformdb.DB.Omit("key").First(channel, "id = ?", channelID).Error
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

// ListAllChannels loads channels for admin/runtime workflows.
func ListAllChannels(startIdx int, num int, selectAll bool, idSort bool, sortOptions ...ChannelSortOptions) ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	order := resolveChannelSortOptions(idSort, sortOptions)
	var err error
	if selectAll {
		err = applyChannelSort(platformdb.DB, order).Find(&channels).Error
	} else {
		err = applyChannelSort(platformdb.DB, order).Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	}
	return channels, err
}

// ListAllChannelSummaries loads every channel without credentials for Root-only
// configuration views that derive routing groups from channel assignments.
func ListAllChannelSummaries() ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	err := applyChannelSort(platformdb.DB.Omit("key"), resolveChannelSortOptions(true, nil)).Find(&channels).Error
	return channels, err
}

// SearchChannels searches channels for admin views.
func SearchChannels(keyword string, group string, modelName string, idSort bool, sortOptions ...ChannelSortOptions) ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	modelsCol := "`models`"
	if platformdb.UsingPostgreSQL {
		modelsCol = `"models"`
	}

	baseURLCol := "`base_url`"
	if platformdb.UsingPostgreSQL {
		baseURLCol = `"base_url"`
	}

	order := resolveChannelSortOptions(idSort, sortOptions)
	baseQuery := platformdb.DB.Model(&gatewayschema.Channel{}).Omit("key")

	whereClause := "(id = ? OR name LIKE ? OR `key` = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
	if platformdb.UsingPostgreSQL {
		whereClause = "(id = ? OR name LIKE ? OR \"key\" = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
	}
	args := []any{platformtext.String2Int(keyword), "%" + keyword + "%", keyword, "%" + keyword + "%", "%" + modelName + "%"}
	baseQuery = ApplyChannelGroupFilter(baseQuery.Where(whereClause, args...), group)

	if err := applyChannelSort(baseQuery, order).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// ListChannelsByTag loads channels under one tag, optionally including sensitive fields.
func ListChannelsByTag(tag string, idSort bool, selectAll bool, sortOptions ...ChannelSortOptions) ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	order := resolveChannelSortOptions(idSort, sortOptions)
	query := applyChannelSort(platformdb.DB.Where("tag = ?", tag), order)
	if !selectAll {
		query = query.Omit("key")
	}
	err := query.Find(&channels).Error
	return channels, err
}

// DeleteDisabledChannels removes disabled channels from storage.
func DeleteDisabledChannels() (int64, error) {
	result := platformdb.DB.Where("status = ? or status = ?", constant.ChannelStatusAutoDisabled, constant.ChannelStatusManuallyDisabled).Delete(&gatewayschema.Channel{})
	return result.RowsAffected, result.Error
}

// EnableChannelsByTag enables all channels and abilities under one tag.
func EnableChannelsByTag(tag string) error {
	if err := platformdb.DB.Model(&gatewayschema.Channel{}).Where("tag = ?", tag).Update("status", constant.ChannelStatusEnabled).Error; err != nil {
		return err
	}
	return updateAbilityStatusByTag(tag, true)
}

// DisableChannelsByTag disables all channels and abilities under one tag.
func DisableChannelsByTag(tag string) error {
	if err := platformdb.DB.Model(&gatewayschema.Channel{}).Where("tag = ?", tag).Update("status", constant.ChannelStatusManuallyDisabled).Error; err != nil {
		return err
	}
	return updateAbilityStatusByTag(tag, false)
}

// EditChannelsByTag updates shared editable fields for all channels under one tag.
func EditChannelsByTag(tag string, newTag *string, modelMapping *string, models *string, group *string, priority *int64, weight *uint, paramOverride *string, headerOverride *string) error {
	updateData := gatewayschema.Channel{}
	shouldReCreateAbilities := false
	updatedTag := tag

	if newTag != nil && *newTag != tag {
		updateData.Tag = newTag
		updatedTag = *newTag
	}
	if modelMapping != nil {
		updateData.ModelMapping = modelMapping
	}
	if models != nil && *models != "" {
		shouldReCreateAbilities = true
		updateData.Models = *models
	}
	if group != nil && *group != "" {
		shouldReCreateAbilities = true
		updateData.Group = *group
	}
	if priority != nil {
		updateData.Priority = priority
	}
	if weight != nil {
		updateData.Weight = weight
	}
	if paramOverride != nil {
		updateData.ParamOverride = paramOverride
	}
	if headerOverride != nil {
		updateData.HeaderOverride = headerOverride
	}

	if err := platformdb.DB.Model(&gatewayschema.Channel{}).Where("tag = ?", tag).Updates(updateData).Error; err != nil {
		return err
	}
	if shouldReCreateAbilities {
		channels, err := ListChannelsByTag(updatedTag, false, false)
		if err == nil {
			for _, channel := range channels {
				err = UpdateChannelAbilities(channel, nil)
				if err != nil {
					platformobservability.SysLog(fmt.Sprintf("failed to update abilities: channel_id=%d, tag=%s, error=%v", channel.Id, channel.GetTag(), err))
				}
			}
		}
		return nil
	}
	return updateAbilityByTag(tag, newTag, priority, weight)
}

// BatchDeleteChannels deletes channels and linked abilities in chunks.
func BatchDeleteChannels(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	tx := platformdb.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	for _, chunk := range lo.Chunk(ids, 200) {
		if err := tx.Where("id in (?)", chunk).Delete(&gatewayschema.Channel{}).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Where("channel_id in (?)", chunk).Delete(&gatewayschema.Ability{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// LoadChannelsByIDs fetches channels by identifiers.
func LoadChannelsByIDs(ids []int) ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	err := platformdb.DB.Where("id in (?)", ids).Find(&channels).Error
	return channels, err
}

// BatchSetChannelTag updates the tag for multiple channels and rebuilds affected abilities.
func BatchSetChannelTag(ids []int, tag *string) error {
	tx := platformdb.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Model(&gatewayschema.Channel{}).Where("id in (?)", ids).Update("tag", tag).Error; err != nil {
		tx.Rollback()
		return err
	}

	channels, err := LoadChannelsByIDs(ids)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, channel := range channels {
		if err := UpdateChannelAbilities(channel, tx); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ListPaginatedChannelTags lists distinct non-empty channel tags from a base query.
func ListPaginatedChannelTags(query *gorm.DB, offset int, limit int) ([]*string, error) {
	var tags []*string
	err := query.
		Select("DISTINCT tag").
		Where("tag is not null AND tag != ''").
		Order(clause.OrderByColumn{Column: clause.Column{Name: "tag"}}).
		Offset(offset).
		Limit(limit).
		Find(&tags).Error
	return tags, err
}

// SearchChannelTags searches matching distinct tags using channel search semantics.
func SearchChannelTags(keyword string, group string, modelName string, idSort bool) ([]*string, error) {
	var tags []*string
	modelsCol := "`models`"
	baseURLCol := "`base_url`"
	keyCol := "`key`"
	if platformdb.UsingPostgreSQL {
		modelsCol = `"models"`
		baseURLCol = `"base_url"`
		keyCol = `"key"`
	}

	order := "priority desc"
	if idSort {
		order = "id desc"
	}

	baseQuery := platformdb.DB.Model(&gatewayschema.Channel{}).Omit("key")
	whereClause := "(id = ? OR name LIKE ? OR " + keyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
	args := []any{platformtext.String2Int(keyword), "%" + keyword + "%", keyword, "%" + keyword + "%", "%" + modelName + "%"}
	baseQuery = ApplyChannelGroupFilter(baseQuery.Where(whereClause, args...), group)

	subQuery := baseQuery.Select("tag").Where("tag != ''").Order(order)
	err := platformdb.DB.Table("(?) as sub", subQuery).Select("DISTINCT tag").Find(&tags).Error
	return tags, err
}

// CountChannelTags counts distinct non-empty tags from a base query.
func CountChannelTags(query *gorm.DB) (int64, error) {
	var total int64
	err := query.Where("tag is not null AND tag != ''").Distinct("tag").Count(&total).Error
	return total, err
}

// InitChannelCache rebuilds the gateway channel runtime cache.
func InitChannelCache() {
	if !platformconfig.MemoryCacheEnabled {
		return
	}

	newChannelIDToChannel := make(map[int]*gatewayschema.Channel)
	var channels []*gatewayschema.Channel
	platformdb.DB.Find(&channels)
	for _, channel := range channels {
		newChannelIDToChannel[channel.Id] = channel
	}

	var abilities []*gatewayschema.Ability
	platformdb.DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}

	newGroupToModelChannels := make(map[string]map[string][]int)
	for group := range groups {
		newGroupToModelChannels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != constant.ChannelStatusEnabled {
			continue
		}
		for _, group := range strings.Split(channel.Group, ",") {
			for _, modelName := range strings.Split(channel.Models, ",") {
				newGroupToModelChannels[group][modelName] = append(newGroupToModelChannels[group][modelName], channel.Id)
			}
		}
	}

	for group, modelChannels := range newGroupToModelChannels {
		for modelName, channelIDs := range modelChannels {
			sort.Slice(channelIDs, func(i, j int) bool {
				return newChannelIDToChannel[channelIDs[i]].GetPriority() > newChannelIDToChannel[channelIDs[j]].GetPriority()
			})
			newGroupToModelChannels[group][modelName] = channelIDs
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroupToModelChannels
	for id, channel := range newChannelIDToChannel {
		if !channel.ChannelInfo.IsMultiKey {
			continue
		}
		channel.Keys = channel.GetKeys()
		if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
			if oldChannel, ok := channelsIDM[id]; ok && oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
			}
		}
	}
	channelsIDM = newChannelIDToChannel
	channelSyncLock.Unlock()

	platformobservability.SysLog("gateway channels synced from database")
}

// SyncChannelCache periodically refreshes the runtime cache.
func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		platformobservability.SysLog("syncing gateway channels from database")
		InitChannelCache()
	}
}

// RebuildChannelAbilities recreates abilities from channels and refreshes runtime cache.
func RebuildChannelAbilities() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	if platformdb.UsingSQLite {
		if err := platformdb.DB.Exec("DELETE FROM abilities").Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		if err := platformdb.DB.Exec("TRUNCATE TABLE abilities").Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}

	var channels []*gatewayschema.Channel
	if err := platformdb.DB.Model(&gatewayschema.Channel{}).Find(&channels).Error; err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}

	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *gatewayschema.Channel, _ int) int { return c.Id })
		if err := platformdb.DB.Where("channel_id IN ?", ids).Delete(&gatewayschema.Ability{}).Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		for _, channel := range chunk {
			if err := AddChannelAbilities(channel, nil); err != nil {
				platformobservability.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}

	InitChannelCache()
	return successCount, failCount, nil
}

// GetRandomSatisfiedChannel picks a channel for the given group/model and retry index.
func GetRandomSatisfiedChannel(group string, modelName string, retry int) (*gatewayschema.Channel, error) {
	if !platformconfig.MemoryCacheEnabled {
		return loadRandomSatisfiedChannelFromDB(group, modelName, retry)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channelIDs := group2model2channels[group][modelName]
	if len(channelIDs) == 0 {
		normalizedModel := FormatMatchingModelName(modelName)
		channelIDs = group2model2channels[group][normalizedModel]
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}
	if len(channelIDs) == 1 {
		if channel, ok := channelsIDM[channelIDs[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelIDs[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelID := range channelIDs {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		uniquePriorities[int(channel.GetPriority())] = true
	}

	sortedPriorities := make([]int, 0, len(uniquePriorities))
	for priority := range uniquePriorities {
		sortedPriorities = append(sortedPriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedPriorities)))
	if retry >= len(sortedPriorities) {
		retry = len(sortedPriorities) - 1
	}
	targetPriority := int64(sortedPriorities[retry])

	sumWeight := 0
	targetChannels := make([]*gatewayschema.Channel, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		if channel.GetPriority() == targetPriority {
			sumWeight += channel.GetWeight()
			targetChannels = append(targetChannels, channel)
		}
	}
	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, modelName, targetPriority))
	}

	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		smoothingFactor = 100
	}

	randomWeight := rand.Intn(sumWeight * smoothingFactor)
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	return nil, errors.New("channel not found")
}

// GetCachedChannel loads a channel from the shared cache when available.
func GetCachedChannel(channelID int) (*gatewayschema.Channel, error) {
	if !platformconfig.MemoryCacheEnabled {
		return LoadChannelByID(channelID, true)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channel, ok := channelsIDM[channelID]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", channelID)
	}
	return channel, nil
}

// GetCachedChannelInfo loads a channel info snapshot from cache when available.
func GetCachedChannelInfo(channelID int) (*gatewayschema.ChannelInfo, error) {
	if !platformconfig.MemoryCacheEnabled {
		channel, err := LoadChannelByID(channelID, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channel, ok := channelsIDM[channelID]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", channelID)
	}
	return &channel.ChannelInfo, nil
}

// GetChannelPollingLock returns or creates a mutex for the given channel ID.
func GetChannelPollingLock(channelID int) *sync.Mutex {
	if lock, exists := channelPollingLocks.Load(channelID); exists {
		return lock.(*sync.Mutex)
	}
	newLock := &sync.Mutex{}
	actual, _ := channelPollingLocks.LoadOrStore(channelID, newLock)
	return actual.(*sync.Mutex)
}

// CleanupChannelPollingLocks removes locks for channels that no longer exist.
func CleanupChannelPollingLocks() {
	var activeChannelIDs []int
	platformdb.DB.Model(&gatewayschema.Channel{}).Pluck("id", &activeChannelIDs)

	activeChannelSet := make(map[int]bool, len(activeChannelIDs))
	for _, id := range activeChannelIDs {
		activeChannelSet[id] = true
	}

	channelPollingLocks.Range(func(key, value interface{}) bool {
		channelID := key.(int)
		if !activeChannelSet[channelID] {
			channelPollingLocks.Delete(channelID)
		}
		return true
	})
}

// GetNextEnabledChannelKey selects the next enabled key for a channel.
func GetNextEnabledChannelKey(channel *gatewayschema.Channel) (string, int, *types.APIError) {
	if channel == nil {
		return "", 0, types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if !channel.ChannelInfo.IsMultiKey {
		return channel.Key, 0, nil
	}

	runtimeChannel := channel
	if platformconfig.MemoryCacheEnabled {
		if cachedChannel, err := GetCachedChannel(channel.Id); err == nil && cachedChannel != nil {
			runtimeChannel = cachedChannel
		}
	}

	keys := runtimeChannel.GetKeys()
	if len(keys) == 0 {
		return "", 0, types.NewError(errors.New("no keys available"), types.ErrorCodeChannelNoAvailableKey)
	}

	lock := GetChannelPollingLock(runtimeChannel.Id)
	lock.Lock()
	defer lock.Unlock()

	statusList := runtimeChannel.ChannelInfo.MultiKeyStatusList
	getStatus := func(idx int) int {
		if statusList == nil {
			return constant.ChannelStatusEnabled
		}
		if status, ok := statusList[idx]; ok {
			return status
		}
		return constant.ChannelStatusEnabled
	}

	enabledIdx := make([]int, 0, len(keys))
	for i := range keys {
		if getStatus(i) == constant.ChannelStatusEnabled {
			enabledIdx = append(enabledIdx, i)
		}
	}
	if len(enabledIdx) == 0 {
		return "", 0, types.NewError(errors.New("no enabled keys"), types.ErrorCodeChannelNoAvailableKey)
	}

	switch runtimeChannel.ChannelInfo.MultiKeyMode {
	case constant.MultiKeyModeRandom:
		selectedIdx := enabledIdx[rand.Intn(len(enabledIdx))]
		return keys[selectedIdx], selectedIdx, nil
	case constant.MultiKeyModePolling:
		start := runtimeChannel.ChannelInfo.MultiKeyPollingIndex
		if start < 0 || start >= len(keys) {
			start = 0
		}
		for i := 0; i < len(keys); i++ {
			idx := (start + i) % len(keys)
			if getStatus(idx) != constant.ChannelStatusEnabled {
				continue
			}
			runtimeChannel.ChannelInfo.MultiKeyPollingIndex = (idx + 1) % len(keys)
			channel.ChannelInfo.MultiKeyPollingIndex = runtimeChannel.ChannelInfo.MultiKeyPollingIndex
			if !platformconfig.MemoryCacheEnabled {
				if err := SaveChannelInfo(runtimeChannel); err != nil {
					return "", 0, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
				}
			}
			return keys[idx], idx, nil
		}
		return keys[enabledIdx[0]], enabledIdx[0], nil
	default:
		return keys[enabledIdx[0]], enabledIdx[0], nil
	}
}

// IsChannelEnabledForGroupModel reports whether the channel can serve the group/model pair.
func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !platformconfig.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}
	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}

	normalizedModel := FormatMatchingModelName(modelName)
	if normalizedModel != "" && normalizedModel != modelName {
		return isChannelIDInList(group2model2channels[group][normalizedModel], channelID)
	}
	return false
}

// IsChannelEnabledForAnyGroupModel reports whether the channel can serve the model in any group.
func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	for _, group := range groups {
		if IsChannelEnabledForGroupModel(group, modelName, channelID) {
			return true
		}
	}
	return false
}

// UpdateChannelStatus updates channel or multi-key runtime status and persists the result.
func UpdateChannelStatus(channelID int, usingKey string, status int, reason string) bool {
	if platformconfig.MemoryCacheEnabled {
		channelStatusLock.Lock()
		defer channelStatusLock.Unlock()

		channelCache, _ := GetCachedChannel(channelID)
		if channelCache == nil {
			return false
		}
		if channelCache.ChannelInfo.IsMultiKey {
			pollingLock := GetChannelPollingLock(channelID)
			pollingLock.Lock()
			handleMultiKeyUpdate(channelCache, usingKey, status, reason)
			pollingLock.Unlock()
		} else {
			if channelCache.Status == status {
				return false
			}
			cacheUpdateChannelStatus(channelID, status)
		}
	}

	shouldUpdateAbilities := false
	defer func() {
		if shouldUpdateAbilities {
			if err := updateAbilityStatus(channelID, status == constant.ChannelStatusEnabled); err != nil {
				platformobservability.SysLog(fmt.Sprintf("failed to update ability status: channel_id=%d, error=%v", channelID, err))
			}
		}
	}()

	channel, err := LoadChannelByID(channelID, true)
	if err != nil {
		return false
	}
	if channel.Status == status {
		return false
	}

	if channel.ChannelInfo.IsMultiKey {
		beforeStatus := channel.Status
		pollingLock := GetChannelPollingLock(channelID)
		pollingLock.Lock()
		handleMultiKeyUpdate(channel, usingKey, status, reason)
		pollingLock.Unlock()
		if beforeStatus != channel.Status {
			shouldUpdateAbilities = true
		}
	} else {
		info := gatewaydomain.GetOtherInfo(channel)
		info["status_reason"] = reason
		info["status_time"] = platformruntime.GetTimestamp()
		gatewaydomain.SetOtherInfo(channel, info)
		channel.Status = status
		shouldUpdateAbilities = true
	}

	if err := SaveChannelWithoutKey(channel); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to update channel status: channel_id=%d, status=%d, error=%v", channel.Id, status, err))
		return false
	}
	return true
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	groupColumn := "`group`"
	if platformdb.UsingPostgreSQL {
		groupColumn = `"group"`
	}

	var count int64
	err := platformdb.DB.Model(&gatewayschema.Ability{}).
		Where(groupColumn+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}

	normalizedModel := FormatMatchingModelName(modelName)
	if normalizedModel == "" || normalizedModel == modelName {
		return false
	}

	count = 0
	err = platformdb.DB.Model(&gatewayschema.Ability{}).
		Where(groupColumn+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalizedModel, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(channelIDs []int, channelID int) bool {
	for _, id := range channelIDs {
		if id == channelID {
			return true
		}
	}
	return false
}

func cacheUpdateChannelStatus(channelID int, status int) {
	if !platformconfig.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()

	channel, ok := channelsIDM[channelID]
	if !ok {
		return
	}
	channel.Status = status
	if status == constant.ChannelStatusEnabled {
		for _, group := range strings.Split(channel.Group, ",") {
			if _, ok := group2model2channels[group]; !ok {
				group2model2channels[group] = make(map[string][]int)
			}
			for _, modelName := range strings.Split(channel.Models, ",") {
				channelIDs := group2model2channels[group][modelName]
				if isChannelIDInList(channelIDs, channelID) {
					continue
				}
				channelIDs = append(channelIDs, channelID)
				sort.Slice(channelIDs, func(i, j int) bool {
					return channelsIDM[channelIDs[i]].GetPriority() > channelsIDM[channelIDs[j]].GetPriority()
				})
				group2model2channels[group][modelName] = channelIDs
			}
		}
		return
	}

	for group, modelChannels := range group2model2channels {
		for modelName, channelIDs := range modelChannels {
			for i, id := range channelIDs {
				if id == channelID {
					group2model2channels[group][modelName] = append(channelIDs[:i], channelIDs[i+1:]...)
					break
				}
			}
		}
	}
}

func handleMultiKeyUpdate(channel *gatewayschema.Channel, usingKey string, status int, reason string) {
	keys := channel.GetKeys()
	if len(keys) == 0 {
		channel.Status = status
		return
	}

	keyIndex := 0
	for i, key := range keys {
		if key == usingKey {
			keyIndex = i
			break
		}
	}
	if channel.ChannelInfo.MultiKeyStatusList == nil {
		channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
	}
	if status == constant.ChannelStatusEnabled {
		delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
	} else {
		channel.ChannelInfo.MultiKeyStatusList[keyIndex] = status
		if channel.ChannelInfo.MultiKeyDisabledReason == nil {
			channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime == nil {
			channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		}
		channel.ChannelInfo.MultiKeyDisabledReason[keyIndex] = reason
		channel.ChannelInfo.MultiKeyDisabledTime[keyIndex] = platformruntime.GetTimestamp()
	}
	if len(channel.ChannelInfo.MultiKeyStatusList) >= channel.ChannelInfo.MultiKeySize {
		channel.Status = constant.ChannelStatusAutoDisabled
		info := gatewaydomain.GetOtherInfo(channel)
		info["status_reason"] = "All keys are disabled"
		info["status_time"] = platformruntime.GetTimestamp()
		gatewaydomain.SetOtherInfo(channel, info)
	}
}

func updateAbilityStatus(channelID int, status bool) error {
	return platformdb.DB.Model(&gatewayschema.Ability{}).Where("channel_id = ?", channelID).Select("enabled").Update("enabled", status).Error
}

var fixLock = sync.Mutex{}
