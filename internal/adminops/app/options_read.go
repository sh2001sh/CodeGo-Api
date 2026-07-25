package app

import (
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	"strings"
)

// ListOptions returns visible runtime options plus the computed completion ratio meta option.
func ListOptions() []*platformschema.Option {
	options := make([]*platformschema.Option, 0, len(platformconfig.OptionMap)+1)
	optionValues := make(map[string]string)

	platformconfig.OptionMapRWMutex.RLock()
	for key, rawValue := range platformconfig.OptionMap {
		value := platformencoding.Interface2String(rawValue)
		isSensitiveKey := strings.HasSuffix(key, "Token") ||
			strings.HasSuffix(key, "Secret") ||
			strings.HasSuffix(key, "Key") ||
			strings.HasSuffix(key, "secret") ||
			strings.HasSuffix(key, "api_key")
		if isSensitiveKey {
			continue
		}

		options = append(options, &platformschema.Option{
			Key:   key,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == key {
				optionValues[key] = value
				break
			}
		}
	}
	platformconfig.OptionMapRWMutex.RUnlock()

	options = append(options, &platformschema.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	return options
}
