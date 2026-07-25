package app

import (
	"errors"
	"strings"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

var (
	ErrVendorIDInvalid      = errors.New("无效的供应商 ID")
	ErrVendorIDRequired     = errors.New("缺少供应商 ID")
	ErrVendorNameRequired   = errors.New("供应商名称不能为空")
	ErrVendorNameDuplicated = errors.New("供应商名称已存在")
)

// GetVendorMeta returns a vendor by ID.
func GetVendorMeta(id int) (*gatewayschema.Vendor, error) {
	if id <= 0 {
		return nil, ErrVendorIDInvalid
	}
	return gatewaystore.LoadVendorByID(id)
}

// ListVendors returns paginated vendors and total count.
func ListVendors(offset int, limit int) ([]*gatewayschema.Vendor, int64, error) {
	vendors, err := gatewaystore.ListVendors(offset, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := gatewaystore.CountVendors()
	if err != nil {
		return nil, 0, err
	}
	return vendors, total, nil
}

// SearchVendors returns paginated vendor search results.
func SearchVendors(keyword string, offset int, limit int) ([]*gatewayschema.Vendor, int64, error) {
	return gatewaystore.SearchVendors(strings.TrimSpace(keyword), offset, limit)
}

// CreateVendorMeta validates and creates a vendor.
func CreateVendorMeta(vendor gatewayschema.Vendor) (*gatewayschema.Vendor, error) {
	vendor.Name = strings.TrimSpace(vendor.Name)
	if vendor.Name == "" {
		return nil, ErrVendorNameRequired
	}
	if duplicated, err := gatewaystore.IsVendorNameDuplicated(0, vendor.Name); err != nil {
		return nil, err
	} else if duplicated {
		return nil, ErrVendorNameDuplicated
	}
	if err := gatewaystore.CreateVendorRecord(&vendor); err != nil {
		return nil, err
	}
	return &vendor, nil
}

// UpdateVendorMeta validates and updates a vendor.
func UpdateVendorMeta(vendor gatewayschema.Vendor) (*gatewayschema.Vendor, error) {
	if vendor.Id <= 0 {
		return nil, ErrVendorIDRequired
	}
	vendor.Name = strings.TrimSpace(vendor.Name)
	if vendor.Name == "" {
		return nil, ErrVendorNameRequired
	}
	if duplicated, err := gatewaystore.IsVendorNameDuplicated(vendor.Id, vendor.Name); err != nil {
		return nil, err
	} else if duplicated {
		return nil, ErrVendorNameDuplicated
	}
	if err := gatewaystore.UpdateVendorRecord(&vendor); err != nil {
		return nil, err
	}
	return &vendor, nil
}

// DeleteVendorMeta removes a vendor by ID.
func DeleteVendorMeta(id int) error {
	if id <= 0 {
		return ErrVendorIDInvalid
	}
	return gatewaystore.DeleteVendorRecord(id)
}
