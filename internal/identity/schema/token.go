package schema

import (
	"strings"

	"gorm.io/gorm"
)

const tokenKeyPrefix = "sk-"

// Token is a user-owned API credential and its quota policy.
type Token struct {
	Id                 int            `json:"id"`
	UserId             int            `json:"user_id" gorm:"index"`
	Key                string         `json:"key" gorm:"type:varchar(128);uniqueIndex"`
	Status             int            `json:"status" gorm:"default:1"`
	Name               string         `json:"name" gorm:"index"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	AccessedTime       int64          `json:"accessed_time" gorm:"bigint"`
	ExpiredTime        int64          `json:"expired_time" gorm:"bigint;default:-1"`
	RemainQuota        int            `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota     bool           `json:"unlimited_quota"`
	ModelLimitsEnabled bool           `json:"model_limits_enabled"`
	ModelLimits        string         `json:"model_limits" gorm:"type:text"`
	AllowIps           *string        `json:"allow_ips" gorm:"default:''"`
	UsedQuota          int            `json:"used_quota" gorm:"default:0"`
	Group              string         `json:"group" gorm:"default:''"`
	CrossGroupRetry    bool           `json:"cross_group_retry"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (token *Token) Clean() {
	token.Key = ""
}

func (token *Token) GetFullKey() string {
	key := strings.TrimSpace(token.Key)
	if key == "" || strings.HasPrefix(key, tokenKeyPrefix) {
		return key
	}
	return tokenKeyPrefix + key
}

func (token *Token) GetMaskedKey() string {
	return maskTokenKey(token.GetFullKey())
}

func (token *Token) GetIpLimits() []string {
	ipLimits := make([]string, 0)
	if token.AllowIps == nil {
		return ipLimits
	}
	cleanIps := strings.ReplaceAll(*token.AllowIps, " ", "")
	if cleanIps == "" {
		return ipLimits
	}
	for _, ip := range strings.Split(cleanIps, "\n") {
		ip = strings.ReplaceAll(strings.TrimSpace(ip), ",", "")
		if ip != "" {
			ipLimits = append(ipLimits, ip)
		}
	}
	return ipLimits
}

func (token *Token) GetModelLimitsMap() map[string]bool {
	limitsMap := make(map[string]bool)
	for _, limit := range strings.Split(token.ModelLimits, ",") {
		if limit != "" {
			limitsMap[limit] = true
		}
	}
	return limitsMap
}

func maskTokenKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	if len(key) <= 8 {
		return key[:2] + "****" + key[len(key)-2:]
	}
	return key[:4] + "**********" + key[len(key)-4:]
}
