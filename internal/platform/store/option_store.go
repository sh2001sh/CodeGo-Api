package store

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"strings"
	"time"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
)

// ListOptions returns all persisted runtime options.
func ListOptions() ([]*platformschema.Option, error) {
	var options []*platformschema.Option
	err := platformdb.DB.Find(&options).Error
	return options, err
}

// InitOptionMap loads default runtime options and overlays persisted values.
func InitOptionMap() {
	platformconfig.OptionMapRWMutex.Lock()
	platformconfig.OptionMap = buildDefaultOptionMap()
	platformconfig.OptionMapRWMutex.Unlock()

	loadOptionsFromDatabase()
	syncDefaultBrandingOptions()
}

func loadOptionsFromDatabase() {
	options, err := ListOptions()
	if err != nil {
		platformobservability.SysLog("failed to load options from database: " + err.Error())
		return
	}
	for _, option := range options {
		if err := applyOptionValue(option.Key, option.Value); err != nil {
			platformobservability.SysLog("failed to update option map: " + err.Error())
		}
	}
}

func syncDefaultBrandingOptions() {
	legacySystemNames := map[string]struct{}{
		"":          {},
		"codegoapi": {},
	}
	legacyLogos := map[string]struct{}{
		"":          {},
		"/logo.png": {},
	}

	if _, ok := legacySystemNames[strings.TrimSpace(platformconfig.SystemName)]; ok {
		_ = UpdateOption("SystemName", "codego-api")
	}
	if _, ok := legacyLogos[strings.TrimSpace(platformconfig.Logo)]; ok {
		_ = UpdateOption("Logo", "/code-go-logo.svg")
	}
}

// SyncOptions periodically refreshes runtime options from the database.
func SyncOptions(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		platformobservability.SysLog("syncing options from database")
		loadOptionsFromDatabase()
	}
}

// UpdateOption persists and applies a runtime option update.
func UpdateOption(key string, value string) error {
	option := platformschema.Option{Key: key}
	if err := platformdb.DB.FirstOrCreate(&option, platformschema.Option{Key: key}).Error; err != nil {
		return err
	}
	option.Value = value
	if err := platformdb.DB.Save(&option).Error; err != nil {
		return err
	}
	return applyOptionValue(key, value)
}
