package app

import (
	"errors"
	"strings"

	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
)

var (
	ErrPrefillGroupIDInvalid          = errors.New("无效的组 ID")
	ErrPrefillGroupIDRequired         = errors.New("缺少组 ID")
	ErrPrefillGroupNameOrTypeRequired = errors.New("组名称和类型不能为空")
	ErrPrefillGroupNameDuplicated     = errors.New("组名称已存在")
)

// ListPrefillGroups returns prefill groups optionally filtered by type.
func ListPrefillGroups(groupType string) ([]*gatewayschema.PrefillGroup, error) {
	return listPrefillGroupRecords(strings.TrimSpace(groupType))
}

// CreatePrefillGroup validates and creates a prefill group.
func CreatePrefillGroup(group gatewayschema.PrefillGroup) (*gatewayschema.PrefillGroup, error) {
	group.Name = strings.TrimSpace(group.Name)
	group.Type = strings.TrimSpace(group.Type)
	if group.Name == "" || group.Type == "" {
		return nil, ErrPrefillGroupNameOrTypeRequired
	}
	if duplicated, err := isPrefillGroupNameDuplicated(0, group.Name); err != nil {
		return nil, err
	} else if duplicated {
		return nil, ErrPrefillGroupNameDuplicated
	}
	if err := createPrefillGroupRecord(&group); err != nil {
		return nil, err
	}
	return &group, nil
}

// UpdatePrefillGroup validates and updates a prefill group.
func UpdatePrefillGroup(group gatewayschema.PrefillGroup) (*gatewayschema.PrefillGroup, error) {
	if group.Id <= 0 {
		return nil, ErrPrefillGroupIDRequired
	}
	group.Name = strings.TrimSpace(group.Name)
	group.Type = strings.TrimSpace(group.Type)
	if group.Name == "" || group.Type == "" {
		return nil, ErrPrefillGroupNameOrTypeRequired
	}
	if duplicated, err := isPrefillGroupNameDuplicated(group.Id, group.Name); err != nil {
		return nil, err
	} else if duplicated {
		return nil, ErrPrefillGroupNameDuplicated
	}
	if err := updatePrefillGroupRecord(&group); err != nil {
		return nil, err
	}
	return &group, nil
}

// DeletePrefillGroup removes a prefill group by ID.
func DeletePrefillGroup(id int) error {
	if id <= 0 {
		return ErrPrefillGroupIDInvalid
	}
	return deletePrefillGroupRecord(id)
}
