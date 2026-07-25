package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

var ErrLoadMissingModels = errors.New("load missing models failed")

// SyncUpstreamModels synchronizes upstream model metadata into local model and vendor records.
func SyncUpstreamModels(ctx context.Context, req SyncRequest) (*SyncUpstreamResult, error) {
	missing, err := gatewaystore.LoadMissingModels()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLoadMissingModels, err)
	}

	source := ResolveUpstreamSource(req.Locale)
	if len(missing) == 0 && len(req.Overwrite) == 0 {
		return &SyncUpstreamResult{
			SkippedModels: []string{},
			CreatedList:   []string{},
			UpdatedList:   []string{},
			Source:        source,
		}, nil
	}

	timeoutSec := platformconfig.GetEnvOrDefaultInt("SYNC_HTTP_TIMEOUT_SECONDS", 15)
	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	vendorByName, modelByName, err := fetchUpstreamData(fetchCtx, source)
	if err != nil {
		return nil, err
	}

	result := &SyncUpstreamResult{
		SkippedModels: make([]string, 0),
		CreatedList:   make([]string, 0),
		UpdatedList:   make([]string, 0),
		Source:        source,
	}
	vendorIDCache := make(map[string]int)

	createMissingModels(missing, modelByName, vendorByName, vendorIDCache, result)
	applyOverwriteFields(req.Overwrite, modelByName, vendorByName, vendorIDCache, result)
	return result, nil
}

// PreviewUpstreamModels previews upstream model conflicts and missing models before synchronization.
func PreviewUpstreamModels(ctx context.Context, locale string) (*UpstreamPreviewResult, error) {
	source := ResolveUpstreamSource(locale)
	timeoutSec := platformconfig.GetEnvOrDefaultInt("SYNC_HTTP_TIMEOUT_SECONDS", 15)
	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	_, modelByName, err := fetchUpstreamData(fetchCtx, source)
	if err != nil {
		return nil, err
	}

	locals, idToVendorName := loadLocalModelsAndVendors(modelByName)
	missing := collectPreviewMissingModels(modelByName)
	conflicts := collectPreviewConflicts(locals, modelByName, idToVendorName)

	return &UpstreamPreviewResult{
		Missing:   missing,
		Conflicts: conflicts,
		Source:    source,
	}, nil
}

func createMissingModels(missing []string, modelByName map[string]upstreamModel, vendorByName map[string]upstreamVendor, vendorIDCache map[string]int, result *SyncUpstreamResult) {
	for _, name := range missing {
		upstreamItem, ok := modelByName[name]
		if !ok {
			result.SkippedModels = append(result.SkippedModels, name)
			continue
		}
		if shouldSkipExistingMissingModel(name) {
			result.SkippedModels = append(result.SkippedModels, name)
			continue
		}

		vendorID := ensureVendorID(upstreamItem.VendorName, vendorByName, vendorIDCache, &result.CreatedVendors)
		item := &gatewayschema.Model{
			ModelName:   name,
			Description: upstreamItem.Description,
			Icon:        upstreamItem.Icon,
			Tags:        upstreamItem.Tags,
			VendorID:    vendorID,
			Status:      chooseStatus(upstreamItem.Status, 1),
			NameRule:    upstreamItem.NameRule,
		}
		if err := gatewaystore.CreateModelRecord(item); err == nil {
			result.CreatedModels++
			result.CreatedList = append(result.CreatedList, name)
		} else {
			result.SkippedModels = append(result.SkippedModels, name)
		}
	}
}

func shouldSkipExistingMissingModel(name string) bool {
	var existing gatewayschema.Model
	if err := platformdb.DB.Where("model_name = ?", name).First(&existing).Error; err != nil {
		return false
	}
	return existing.SyncOfficial == 0
}

func applyOverwriteFields(overwrites []OverwriteField, modelByName map[string]upstreamModel, vendorByName map[string]upstreamVendor, vendorIDCache map[string]int, result *SyncUpstreamResult) {
	for _, overwrite := range overwrites {
		upstreamItem, ok := modelByName[overwrite.ModelName]
		if !ok {
			continue
		}
		var local gatewayschema.Model
		if err := platformdb.DB.Where("model_name = ?", overwrite.ModelName).First(&local).Error; err != nil {
			continue
		}
		if local.SyncOfficial == 0 {
			continue
		}

		newVendorID := ensureVendorID(upstreamItem.VendorName, vendorByName, vendorIDCache, &result.CreatedVendors)
		if updated := updateLocalModelFromUpstream(&local, overwrite.Fields, upstreamItem, newVendorID); !updated {
			continue
		}
		if err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
			return tx.Save(&local).Error
		}); err == nil {
			result.UpdatedModels++
			result.UpdatedList = append(result.UpdatedList, overwrite.ModelName)
		}
	}
}

func updateLocalModelFromUpstream(local *gatewayschema.Model, fields []string, upstreamItem upstreamModel, newVendorID int) bool {
	updated := false
	if containsField(fields, "description") {
		local.Description = upstreamItem.Description
		updated = true
	}
	if containsField(fields, "icon") {
		local.Icon = upstreamItem.Icon
		updated = true
	}
	if containsField(fields, "tags") {
		local.Tags = upstreamItem.Tags
		updated = true
	}
	if containsField(fields, "vendor") {
		local.VendorID = newVendorID
		updated = true
	}
	if containsField(fields, "name_rule") {
		local.NameRule = upstreamItem.NameRule
		updated = true
	}
	if containsField(fields, "status") {
		local.Status = chooseStatus(upstreamItem.Status, local.Status)
		updated = true
	}
	return updated
}

func ensureVendorID(vendorName string, vendorByName map[string]upstreamVendor, vendorIDCache map[string]int, createdVendors *int) int {
	if vendorName == "" {
		return 0
	}
	if id, ok := vendorIDCache[vendorName]; ok {
		return id
	}

	existing, err := gatewaystore.LoadVendorByName(vendorName)
	if err == nil {
		vendorIDCache[vendorName] = existing.Id
		return existing.Id
	}

	upstreamVendor := vendorByName[vendorName]
	vendor := &gatewayschema.Vendor{
		Name:        vendorName,
		Description: upstreamVendor.Description,
		Icon:        coalesce(upstreamVendor.Icon, ""),
		Status:      chooseStatus(upstreamVendor.Status, 1),
	}
	if err := gatewaystore.CreateVendorRecord(vendor); err == nil {
		*createdVendors++
		vendorIDCache[vendorName] = vendor.Id
		return vendor.Id
	}

	vendorIDCache[vendorName] = 0
	return 0
}

func loadLocalModelsAndVendors(modelByName map[string]upstreamModel) ([]gatewayschema.Model, map[int]string) {
	upstreamNames := make([]string, 0, len(modelByName))
	for name := range modelByName {
		upstreamNames = append(upstreamNames, name)
	}

	var locals []gatewayschema.Model
	if len(upstreamNames) > 0 {
		_ = platformdb.DB.Where("model_name IN ? AND sync_official <> 0", upstreamNames).Find(&locals).Error
	}

	vendorIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, item := range locals {
		if item.VendorID != 0 {
			if _, ok := seen[item.VendorID]; !ok {
				seen[item.VendorID] = struct{}{}
				vendorIDs = append(vendorIDs, item.VendorID)
			}
		}
	}

	idToVendorName := make(map[int]string)
	if len(vendorIDs) > 0 {
		var vendors []gatewayschema.Vendor
		_ = platformdb.DB.Where("id IN ?", vendorIDs).Find(&vendors).Error
		for _, vendor := range vendors {
			idToVendorName[vendor.Id] = vendor.Name
		}
	}
	return locals, idToVendorName
}

func collectPreviewMissingModels(modelByName map[string]upstreamModel) []string {
	missingList, _ := gatewaystore.LoadMissingModels()
	missing := make([]string, 0)
	for _, name := range missingList {
		if _, ok := modelByName[name]; ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func collectPreviewConflicts(locals []gatewayschema.Model, modelByName map[string]upstreamModel, idToVendorName map[int]string) []UpstreamConflictItem {
	conflicts := make([]UpstreamConflictItem, 0)
	for _, local := range locals {
		upstreamItem, ok := modelByName[local.ModelName]
		if !ok {
			continue
		}
		fields := compareConflictFields(local, upstreamItem, idToVendorName[local.VendorID])
		if len(fields) > 0 {
			conflicts = append(conflicts, UpstreamConflictItem{
				ModelName: local.ModelName,
				Fields:    fields,
			})
		}
	}
	return conflicts
}

func compareConflictFields(local gatewayschema.Model, upstreamItem upstreamModel, localVendor string) []UpstreamConflictField {
	fields := make([]UpstreamConflictField, 0, 6)
	if strings.TrimSpace(local.Description) != strings.TrimSpace(upstreamItem.Description) {
		fields = append(fields, UpstreamConflictField{Field: "description", Local: local.Description, Upstream: upstreamItem.Description})
	}
	if strings.TrimSpace(local.Icon) != strings.TrimSpace(upstreamItem.Icon) {
		fields = append(fields, UpstreamConflictField{Field: "icon", Local: local.Icon, Upstream: upstreamItem.Icon})
	}
	if strings.TrimSpace(local.Tags) != strings.TrimSpace(upstreamItem.Tags) {
		fields = append(fields, UpstreamConflictField{Field: "tags", Local: local.Tags, Upstream: upstreamItem.Tags})
	}
	if strings.TrimSpace(localVendor) != strings.TrimSpace(upstreamItem.VendorName) {
		fields = append(fields, UpstreamConflictField{Field: "vendor", Local: localVendor, Upstream: upstreamItem.VendorName})
	}
	if local.NameRule != upstreamItem.NameRule {
		fields = append(fields, UpstreamConflictField{Field: "name_rule", Local: local.NameRule, Upstream: upstreamItem.NameRule})
	}
	if local.Status != chooseStatus(upstreamItem.Status, local.Status) {
		fields = append(fields, UpstreamConflictField{Field: "status", Local: local.Status, Upstream: upstreamItem.Status})
	}
	return fields
}

func containsField(fields []string, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, field := range fields {
		if strings.ToLower(strings.TrimSpace(field)) == key {
			return true
		}
	}
	return false
}

func coalesce(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func chooseStatus(primary int, fallback int) int {
	if primary == 0 && fallback != 0 {
		return fallback
	}
	if primary != 0 {
		return primary
	}
	return 1
}
