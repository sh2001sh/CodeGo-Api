package store

import (
	"strconv"
	"strings"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

func LoadMissingModels() ([]string, error) {
	models := LoadEnabledModels()
	if len(models) == 0 {
		return []string{}, nil
	}

	var existing []string
	if err := platformdb.DB.Model(&gatewayschema.Model{}).Where("model_name IN ?", models).Pluck("model_name", &existing).Error; err != nil {
		return nil, err
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		existingSet[name] = struct{}{}
	}

	missing := make([]string, 0, len(models))
	for _, name := range models {
		if _, ok := existingSet[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

func EnsureEnabledModelsMeta() error {
	missing, err := LoadMissingModels()
	if err != nil {
		return err
	}

	for _, modelName := range missing {
		if strings.TrimSpace(modelName) == "" {
			continue
		}

		var existing gatewayschema.Model
		if err := platformdb.DB.Where("model_name = ?", modelName).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		endpoints := LoadModelSupportedEndpointTypes(modelName)
		endpointsJSON := ""
		if len(endpoints) > 0 {
			if data, marshalErr := platformencoding.Marshal(endpoints); marshalErr == nil {
				endpointsJSON = string(data)
			}
		}

		placeholder := &gatewayschema.Model{
			ModelName:    modelName,
			Endpoints:    endpointsJSON,
			Status:       1,
			SyncOfficial: 1,
			NameRule:     gatewayschema.NameRuleExact,
		}
		if err := CreateModelRecord(placeholder); err != nil {
			duplicated, dupErr := IsModelNameDuplicated(0, modelName)
			if dupErr == nil && duplicated {
				continue
			}
			return err
		}
	}

	return nil
}

func LoadModelByID(id int) (*gatewayschema.Model, error) {
	var item gatewayschema.Model
	if err := platformdb.DB.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func CreateModelRecord(item *gatewayschema.Model) error {
	now := platformruntime.GetTimestamp()
	item.CreatedTime = now
	item.UpdatedTime = now

	originalStatus := item.Status
	originalSyncOfficial := item.SyncOfficial

	if err := platformdb.DB.Create(item).Error; err != nil {
		return err
	}

	return platformdb.DB.Model(&gatewayschema.Model{}).Where("id = ?", item.Id).Updates(map[string]any{
		"status":        originalStatus,
		"sync_official": originalSyncOfficial,
	}).Error
}

func UpdateModelRecord(item *gatewayschema.Model) error {
	item.UpdatedTime = platformruntime.GetTimestamp()
	return platformdb.DB.Model(&gatewayschema.Model{}).Where("id = ?", item.Id).
		Select("model_name", "description", "icon", "tags", "vendor_id", "endpoints", "status", "sync_official", "name_rule", "updated_time").
		Updates(item).Error
}

func UpdateModelStatus(id int, status int) error {
	return platformdb.DB.Model(&gatewayschema.Model{}).Where("id = ?", id).Update("status", status).Error
}

func DeleteModelRecord(id int) error {
	return platformdb.DB.Delete(&gatewayschema.Model{}, id).Error
}

func CountModels(status string, syncOfficial string) (int64, error) {
	var total int64
	err := applyModelMetaFilters(platformdb.DB.Model(&gatewayschema.Model{}), status, syncOfficial).
		Count(&total).Error
	return total, err
}

func ListModels(offset int, limit int, status string, syncOfficial string) ([]*gatewayschema.Model, error) {
	var models []*gatewayschema.Model
	err := applyModelMetaFilters(platformdb.DB.Model(&gatewayschema.Model{}), status, syncOfficial).
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error
	return models, err
}

func LoadVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := platformdb.DB.Model(&gatewayschema.Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}

	result := make(map[int64]int64, len(stats))
	for _, stat := range stats {
		result[stat.VendorID] = stat.Count
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, status string, syncOfficial string, offset int, limit int) ([]*gatewayschema.Model, int64, error) {
	var models []*gatewayschema.Model
	db := applyModelMetaFilters(platformdb.DB.Model(&gatewayschema.Model{}), status, syncOfficial)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if vendor != "" {
		if vendorID, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vendorID)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	var count int64
	err := platformdb.DB.Model(&gatewayschema.Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&count).Error
	return count > 0, err
}

func LoadBoundChannelsByModelsMap(modelNames []string) (map[string][]gatewayschema.BoundChannel, error) {
	result := make(map[string][]gatewayschema.BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}

	type row struct {
		Model string
		Name  string
		Type  int
	}

	var rows []row
	err := platformdb.DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.Model] = append(result[row.Model], gatewayschema.BoundChannel{Name: row.Name, Type: row.Type})
	}
	return result, nil
}

func applyModelMetaFilters(db *gorm.DB, status string, syncOfficial string) *gorm.DB {
	if statusValue := normalizeModelStatusFilter(status); statusValue != nil {
		db = db.Where("status = ?", *statusValue)
	}
	if syncValue := normalizeModelSyncFilter(syncOfficial); syncValue != nil {
		db = db.Where("sync_official = ?", *syncValue)
	}
	return db
}

func normalizeModelStatusFilter(status string) *int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return nil
	case "enabled", "1", "true", "yes":
		value := 1
		return &value
	case "disabled", "0", "false", "no":
		value := 0
		return &value
	default:
		return nil
	}
}

func normalizeModelSyncFilter(syncOfficial string) *int {
	switch strings.ToLower(strings.TrimSpace(syncOfficial)) {
	case "", "all":
		return nil
	case "yes", "enabled", "1", "true":
		value := 1
		return &value
	case "no", "disabled", "0", "false":
		value := 0
		return &value
	default:
		return nil
	}
}
