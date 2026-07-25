package domain

import (
	"encoding/json"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"

	"github.com/sh2001sh/CodeGo-Api/dto"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

func GetSetting(user *identityschema.User) dto.UserSetting {
	setting := dto.UserSetting{}
	if user == nil || user.Setting == "" {
		return setting
	}
	if err := json.Unmarshal([]byte(user.Setting), &setting); err != nil {
		platformobservability.SysLog("failed to unmarshal setting: " + err.Error())
	}
	return setting
}

func SetSetting(user *identityschema.User, setting dto.UserSetting) {
	if user == nil {
		return
	}
	settingBytes, err := json.Marshal(setting)
	if err != nil {
		platformobservability.SysLog("failed to marshal setting: " + err.Error())
		return
	}
	user.Setting = string(settingBytes)
}

func GetBaseSetting(user *identityschema.UserBase) dto.UserSetting {
	setting := dto.UserSetting{}
	if user == nil || user.Setting == "" {
		return setting
	}
	if err := platformencoding.Unmarshal([]byte(user.Setting), &setting); err != nil {
		platformobservability.SysLog("failed to unmarshal setting: " + err.Error())
	}
	return setting
}
