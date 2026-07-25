package store

import (
	"strings"

	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func AddChannelAbilities(channel *gatewayschema.Channel, tx *gorm.DB) error {
	abilities := buildChannelAbilities(channel)
	if len(abilities) == 0 {
		return nil
	}

	useDB := platformdb.DB
	if tx != nil {
		useDB = tx
	}

	for _, chunk := range lo.Chunk(abilities, 50) {
		if err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error; err != nil {
			return err
		}
	}
	return nil
}

func DeleteChannelAbilities(channelID int) error {
	return platformdb.DB.Where("channel_id = ?", channelID).Delete(&gatewayschema.Ability{}).Error
}

func UpdateChannelAbilities(channel *gatewayschema.Channel, tx *gorm.DB) error {
	isNewTx := false
	if tx == nil {
		tx = platformdb.DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	if err := tx.Where("channel_id = ?", channel.Id).Delete(&gatewayschema.Ability{}).Error; err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	if err := AddChannelAbilities(channel, tx); err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	if isNewTx {
		return tx.Commit().Error
	}
	return nil
}

func buildChannelAbilities(channel *gatewayschema.Channel) []gatewayschema.Ability {
	models := strings.Split(channel.Models, ",")
	groups := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]gatewayschema.Ability, 0, len(models))

	for _, modelName := range models {
		for _, groupName := range groups {
			key := groupName + "|" + modelName
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			abilities = append(abilities, gatewayschema.Ability{
				Group:     groupName,
				Model:     modelName,
				ChannelId: channel.Id,
				Enabled:   channel.Status == constant.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			})
		}
	}

	return abilities
}
