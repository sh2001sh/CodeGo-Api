package app

import (
	"errors"
	"fmt"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"strings"
)

type accessPolicyPayload struct {
	Logic      string                `json:"logic"`
	Conditions []accessConditionItem `json:"conditions"`
	Groups     []accessPolicyPayload `json:"groups"`
}

type accessConditionItem struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

var supportedAccessPolicyOps = map[string]struct{}{
	"eq":           {},
	"ne":           {},
	"gt":           {},
	"gte":          {},
	"lt":           {},
	"lte":          {},
	"in":           {},
	"not_in":       {},
	"contains":     {},
	"not_contains": {},
	"exists":       {},
	"not_exists":   {},
}

func getCustomOAuthProviderRecordByID(id int) (*identitydomain.CustomOAuthProvider, error) {
	var provider identitydomain.CustomOAuthProvider
	if err := platformdb.DB.First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func listCustomOAuthProviderRecords() ([]*identitydomain.CustomOAuthProvider, error) {
	var providers []*identitydomain.CustomOAuthProvider
	err := platformdb.DB.Order("id asc").Find(&providers).Error
	return providers, err
}

func isCustomOAuthProviderSlugTaken(slug string, excludeID int) bool {
	var count int64
	query := platformdb.DB.Model(&identitydomain.CustomOAuthProvider{}).Where("slug = ?", slug)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func createCustomOAuthProviderRecord(provider *identitydomain.CustomOAuthProvider) error {
	if err := validateCustomOAuthProviderRecord(provider); err != nil {
		return err
	}
	return platformdb.DB.Create(provider).Error
}

func updateCustomOAuthProviderRecord(provider *identitydomain.CustomOAuthProvider) error {
	if err := validateCustomOAuthProviderRecord(provider); err != nil {
		return err
	}
	return platformdb.DB.Save(provider).Error
}

func deleteCustomOAuthProviderRecord(id int) error {
	return platformdb.DB.Delete(&identitydomain.CustomOAuthProvider{}, id).Error
}

func countUserOAuthBindingsByProviderID(providerID int) (int64, error) {
	var count int64
	err := platformdb.DB.Model(&identitydomain.UserOAuthBinding{}).Where("provider_id = ?", providerID).Count(&count).Error
	return count, err
}

func validateCustomOAuthProviderRecord(provider *identitydomain.CustomOAuthProvider) error {
	if provider.Name == "" {
		return errors.New("provider name is required")
	}
	if provider.Slug == "" {
		return errors.New("provider slug is required")
	}

	slug := strings.ToLower(provider.Slug)
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return errors.New("provider slug must contain only lowercase letters, numbers, and hyphens")
		}
	}
	provider.Slug = slug

	if provider.ClientId == "" {
		return errors.New("client ID is required")
	}
	if provider.AuthorizationEndpoint == "" {
		return errors.New("authorization endpoint is required")
	}
	if provider.TokenEndpoint == "" {
		return errors.New("token endpoint is required")
	}
	if provider.UserInfoEndpoint == "" {
		return errors.New("user info endpoint is required")
	}

	if provider.UserIdField == "" {
		provider.UserIdField = "sub"
	}
	if provider.UsernameField == "" {
		provider.UsernameField = "preferred_username"
	}
	if provider.DisplayNameField == "" {
		provider.DisplayNameField = "name"
	}
	if provider.EmailField == "" {
		provider.EmailField = "email"
	}
	if provider.Scopes == "" {
		provider.Scopes = "openid profile email"
	}
	if strings.TrimSpace(provider.AccessPolicy) != "" {
		var policy accessPolicyPayload
		if err := platformencoding.UnmarshalString(provider.AccessPolicy, &policy); err != nil {
			return errors.New("access_policy must be valid JSON")
		}
		if err := validateAccessPolicyPayload(&policy); err != nil {
			return fmt.Errorf("access_policy is invalid: %w", err)
		}
	}

	return nil
}

func validateAccessPolicyPayload(policy *accessPolicyPayload) error {
	if policy == nil {
		return errors.New("policy is nil")
	}

	logic := strings.ToLower(strings.TrimSpace(policy.Logic))
	if logic == "" {
		logic = "and"
	}
	if logic != "and" && logic != "or" {
		return fmt.Errorf("unsupported logic: %s", logic)
	}

	if len(policy.Conditions) == 0 && len(policy.Groups) == 0 {
		return errors.New("policy requires at least one condition or group")
	}

	for index, condition := range policy.Conditions {
		field := strings.TrimSpace(condition.Field)
		if field == "" {
			return fmt.Errorf("condition[%d].field is required", index)
		}
		op := strings.ToLower(strings.TrimSpace(condition.Op))
		if _, ok := supportedAccessPolicyOps[op]; !ok {
			return fmt.Errorf("condition[%d].op is unsupported: %s", index, op)
		}
		if op == "in" || op == "not_in" {
			if _, ok := condition.Value.([]any); !ok {
				return fmt.Errorf("condition[%d].value must be an array for op %s", index, op)
			}
		}
	}

	for index := range policy.Groups {
		if err := validateAccessPolicyPayload(&policy.Groups[index]); err != nil {
			return fmt.Errorf("group[%d]: %w", index, err)
		}
	}

	return nil
}
