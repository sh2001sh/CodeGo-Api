package store

import (
	"errors"
	"github.com/samber/lo"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"

	"gorm.io/gorm"
	"strings"
)

func NormalizeChannelGroupFilter(group string) string {
	group = strings.TrimSpace(group)
	if group == "" || strings.EqualFold(group, "all") || strings.EqualFold(group, "null") {
		return ""
	}
	return group
}

func ApplyChannelGroupFilter(query *gorm.DB, group string) *gorm.DB {
	group = NormalizeChannelGroupFilter(group)
	if group == "" {
		return query
	}
	return query.Where(channelGroupFilterCondition(), channelGroupFilterPattern(group))
}

func BatchInsertChannels(channels []gatewayschema.Channel) error {
	if len(channels) == 0 {
		return nil
	}

	tx := platformdb.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, chunk := range lo.Chunk(channels, 50) {
		if err := tx.Create(&chunk).Error; err != nil {
			tx.Rollback()
			return err
		}
		for index := range chunk {
			if err := AddChannelAbilities(&chunk[index], tx); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit().Error
}

func updateAbilityStatusByTag(tag string, enabled bool) error {
	return platformdb.DB.Model(&gatewayschema.Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", enabled).Error
}

func updateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := gatewayschema.Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return platformdb.DB.Model(&gatewayschema.Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

func loadRandomSatisfiedChannelFromDB(group string, modelName string, retry int) (*gatewayschema.Channel, error) {
	channelQuery, err := buildChannelQuery(group, modelName, retry)
	if err != nil {
		return nil, err
	}

	var abilities []gatewayschema.Ability
	if err := channelQuery.Order("weight DESC").Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	channel := gatewayschema.Channel{}
	weightSum := uint(0)
	for _, ability := range abilities {
		weightSum += ability.Weight + 10
	}
	weight := platformruntime.GetRandomInt(int(weightSum))
	for _, ability := range abilities {
		weight -= int(ability.Weight) + 10
		if weight <= 0 {
			channel.Id = ability.ChannelId
			break
		}
	}
	if channel.Id == 0 {
		channel.Id = abilities[0].ChannelId
	}
	if err := platformdb.DB.First(&channel, "id = ?", channel.Id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func buildChannelQuery(group string, modelName string, retry int) (*gorm.DB, error) {
	groupColumn := abilityGroupColumn()
	maxPrioritySubQuery := platformdb.DB.Model(&gatewayschema.Ability{}).
		Select("MAX(priority)").
		Where(groupColumn+" = ? and model = ? and enabled = ?", group, modelName, true)
	channelQuery := platformdb.DB.Where(groupColumn+" = ? and model = ? and enabled = ? and priority = (?)", group, modelName, true, maxPrioritySubQuery)
	if retry == 0 {
		return channelQuery, nil
	}

	priority, err := loadChannelPriority(group, modelName, retry)
	if err != nil {
		return nil, err
	}
	return platformdb.DB.Where(groupColumn+" = ? and model = ? and enabled = ? and priority = ?", group, modelName, true, priority), nil
}

func loadChannelPriority(group string, modelName string, retry int) (int, error) {
	groupColumn := abilityGroupColumn()

	var priorities []int
	err := platformdb.DB.Model(&gatewayschema.Ability{}).
		Select("DISTINCT(priority)").
		Where(groupColumn+" = ? and model = ? and enabled = ?", group, modelName, true).
		Order("priority DESC").
		Pluck("priority", &priorities).Error
	if err != nil {
		return 0, err
	}
	if len(priorities) == 0 {
		return 0, errors.New("数据库一致性被破坏")
	}
	if retry >= len(priorities) {
		return priorities[len(priorities)-1], nil
	}
	return priorities[retry], nil
}

func resolveChannelSortOptions(idSort bool, sortOptions []ChannelSortOptions) ChannelSortOptions {
	if len(sortOptions) == 0 {
		return NewChannelSortOptions("", "", idSort)
	}
	options := sortOptions[0]
	options.IDSort = options.IDSort || idSort
	return options
}

func channelGroupFilterCondition() string {
	groupColumn := channelGroupColumn()
	if platformdb.UsingMySQL {
		return "CONCAT(',', " + groupColumn + ", ',') LIKE ? ESCAPE '!'"
	}
	return "(',' || " + groupColumn + " || ',') LIKE ? ESCAPE '!'"
}

func channelGroupFilterPattern(group string) string {
	group = strings.NewReplacer(
		"!", "!!",
		"%", "!%",
		"_", "!_",
	).Replace(group)
	return "%," + group + ",%"
}

func channelGroupColumn() string {
	if platformdb.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

func abilityGroupColumn() string {
	return channelGroupColumn()
}

func applyChannelSort(query *gorm.DB, options ChannelSortOptions) *gorm.DB {
	return options.Apply(query)
}
