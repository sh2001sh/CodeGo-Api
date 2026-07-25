package app

import (
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

func listPrefillGroupRecords(groupType string) ([]*gatewayschema.PrefillGroup, error) {
	var groups []*gatewayschema.PrefillGroup
	query := platformdb.DB.Model(&gatewayschema.PrefillGroup{})
	if groupType != "" {
		query = query.Where("type = ?", groupType)
	}
	if err := query.Order("updated_time DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func createPrefillGroupRecord(group *gatewayschema.PrefillGroup) error {
	now := platformruntime.GetTimestamp()
	group.CreatedTime = now
	group.UpdatedTime = now
	return platformdb.DB.Create(group).Error
}

func updatePrefillGroupRecord(group *gatewayschema.PrefillGroup) error {
	group.UpdatedTime = platformruntime.GetTimestamp()
	return platformdb.DB.Save(group).Error
}

func isPrefillGroupNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var count int64
	err := platformdb.DB.Model(&gatewayschema.PrefillGroup{}).Where("name = ? AND id <> ?", name, id).Count(&count).Error
	return count > 0, err
}

func deletePrefillGroupRecord(id int) error {
	return platformdb.DB.Delete(&gatewayschema.PrefillGroup{}, id).Error
}
