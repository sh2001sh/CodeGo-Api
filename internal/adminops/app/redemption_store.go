package app

import (
	"strconv"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func listRedemptionRecords(offset int, limit int) ([]*commerceschema.Redemption, int64, error) {
	var (
		redemptions []*commerceschema.Redemption
		total       int64
	)
	if err := platformdb.DB.Model(&commerceschema.Redemption{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := platformdb.DB.Order("id desc").Limit(limit).Offset(offset).Find(&redemptions).Error; err != nil {
		return nil, 0, err
	}
	return redemptions, total, nil
}

func searchRedemptionRecords(keyword string, offset int, limit int) ([]*commerceschema.Redemption, int64, error) {
	var (
		redemptions []*commerceschema.Redemption
		total       int64
	)

	query := platformdb.DB.Model(&commerceschema.Redemption{})
	trimmed := strings.TrimSpace(keyword)
	if trimmed != "" {
		nameKeyword := "%" + trimmed + "%"
		if id, err := strconv.Atoi(trimmed); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, nameKeyword)
		} else {
			query = query.Where("name LIKE ?", nameKeyword)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(limit).Offset(offset).Find(&redemptions).Error; err != nil {
		return nil, 0, err
	}
	return redemptions, total, nil
}

func loadRedemptionRecord(id int) (*commerceschema.Redemption, error) {
	var redemption commerceschema.Redemption
	if err := platformdb.DB.First(&redemption, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &redemption, nil
}

func createRedemptionRecord(redemption *commerceschema.Redemption) error {
	return platformdb.DB.Create(redemption).Error
}

func updateRedemptionRecord(redemption *commerceschema.Redemption, statusOnly bool) error {
	if statusOnly {
		return platformdb.DB.Model(redemption).Select("status").Updates(redemption).Error
	}

	return platformdb.DB.Model(redemption).Select(
		"name",
		"status",
		"redeem_type",
		"quota",
		"plan_id",
		"plan_title",
		"redeemed_time",
		"expired_time",
	).Updates(redemption).Error
}

func deleteRedemptionRecord(id int) error {
	return platformdb.DB.Delete(&commerceschema.Redemption{}, id).Error
}

func deleteInvalidRedemptionRecords(now int64) (int64, error) {
	result := platformdb.DB.Where(
		"status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)",
		[]int{constant.RedemptionCodeStatusUsed, constant.RedemptionCodeStatusDisabled},
		constant.RedemptionCodeStatusEnabled,
		now,
	).Delete(&commerceschema.Redemption{})
	return result.RowsAffected, result.Error
}
