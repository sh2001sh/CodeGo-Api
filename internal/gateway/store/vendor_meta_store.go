package store

import (
	"strings"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

func LoadVendorByID(id int) (*gatewayschema.Vendor, error) {
	var vendor gatewayschema.Vendor
	if err := platformdb.DB.First(&vendor, id).Error; err != nil {
		return nil, err
	}
	return &vendor, nil
}

func LoadVendorByName(name string) (*gatewayschema.Vendor, error) {
	var vendor gatewayschema.Vendor
	if err := platformdb.DB.Where("name = ?", name).First(&vendor).Error; err != nil {
		return nil, err
	}
	return &vendor, nil
}

func ListVendors(offset int, limit int) ([]*gatewayschema.Vendor, error) {
	var vendors []*gatewayschema.Vendor
	if err := platformdb.DB.Offset(offset).Limit(limit).Find(&vendors).Error; err != nil {
		return nil, err
	}
	return vendors, nil
}

func CountVendors() (int64, error) {
	var total int64
	if err := platformdb.DB.Model(&gatewayschema.Vendor{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func SearchVendors(keyword string, offset int, limit int) ([]*gatewayschema.Vendor, int64, error) {
	query := platformdb.DB.Model(&gatewayschema.Vendor{})
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var vendors []*gatewayschema.Vendor
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&vendors).Error; err != nil {
		return nil, 0, err
	}
	return vendors, total, nil
}

func IsVendorNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	var count int64
	err := platformdb.DB.Model(&gatewayschema.Vendor{}).Where("name = ? AND id <> ?", name, id).Count(&count).Error
	return count > 0, err
}

func CreateVendorRecord(vendor *gatewayschema.Vendor) error {
	now := platformruntime.GetTimestamp()
	vendor.CreatedTime = now
	vendor.UpdatedTime = now
	return platformdb.DB.Create(vendor).Error
}

func UpdateVendorRecord(vendor *gatewayschema.Vendor) error {
	vendor.UpdatedTime = platformruntime.GetTimestamp()
	return platformdb.DB.Save(vendor).Error
}

func DeleteVendorRecord(id int) error {
	return platformdb.DB.Delete(&gatewayschema.Vendor{}, id).Error
}
