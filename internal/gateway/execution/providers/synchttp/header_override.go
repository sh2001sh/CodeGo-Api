package synchttp

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
)

const clientHeaderPlaceholderPrefix = "{client_header:"

const (
	headerPassthroughAllKey        = "*"
	headerPassthroughRegexPrefix   = "re:"
	headerPassthroughRegexPrefixV2 = "regex:"
)

var passthroughSkipHeaderNamesLower = map[string]struct{}{
	"connection":               {},
	"keep-alive":               {},
	"proxy-authenticate":       {},
	"proxy-authorization":      {},
	"te":                       {},
	"trailer":                  {},
	"transfer-encoding":        {},
	"upgrade":                  {},
	"cookie":                   {},
	"host":                     {},
	"content-length":           {},
	"accept-encoding":          {},
	"authorization":            {},
	"x-api-key":                {},
	"x-goog-api-key":           {},
	"sec-websocket-key":        {},
	"sec-websocket-version":    {},
	"sec-websocket-extensions": {},
}

var headerPassthroughRegexCache sync.Map

func getHeaderPassthroughRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, errors.New("empty regex pattern")
	}
	if v, ok := headerPassthroughRegexCache.Load(pattern); ok {
		if re, ok := v.(*regexp.Regexp); ok {
			return re, nil
		}
		headerPassthroughRegexCache.Delete(pattern)
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := headerPassthroughRegexCache.LoadOrStore(pattern, compiled)
	if re, ok := actual.(*regexp.Regexp); ok {
		return re, nil
	}
	return compiled, nil
}

func isHeaderPassthroughRuleKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if key == headerPassthroughAllKey {
		return true
	}
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, headerPassthroughRegexPrefix) || strings.HasPrefix(lower, headerPassthroughRegexPrefixV2)
}

func shouldSkipPassthroughHeader(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	_, ok := passthroughSkipHeaderNamesLower[strings.ToLower(name)]
	return ok
}

func applyHeaderOverridePlaceholders(template string, c *gin.Context, apiKey string) (string, bool, error) {
	trimmed := strings.TrimSpace(template)
	if strings.HasPrefix(trimmed, clientHeaderPlaceholderPrefix) {
		afterPrefix := trimmed[len(clientHeaderPlaceholderPrefix):]
		end := strings.Index(afterPrefix, "}")
		if end < 0 || end != len(afterPrefix)-1 {
			return "", false, fmt.Errorf("client_header placeholder must be the full value: %q", template)
		}

		name := strings.TrimSpace(afterPrefix[:end])
		if name == "" {
			return "", false, fmt.Errorf("client_header placeholder name is empty: %q", template)
		}
		if c == nil || c.Request == nil {
			return "", false, fmt.Errorf("missing request context for client_header placeholder")
		}
		clientHeaderValue := c.Request.Header.Get(name)
		if strings.TrimSpace(clientHeaderValue) == "" {
			return "", false, nil
		}
		return clientHeaderValue, true, nil
	}

	if strings.Contains(template, "{api_key}") {
		template = strings.ReplaceAll(template, "{api_key}", apiKey)
	}
	if strings.TrimSpace(template) == "" {
		return "", false, nil
	}
	return template, true, nil
}

func resolveHeaderOverride(info *relaycommon.RelayInfo, c *gin.Context) (map[string]string, error) {
	headerOverride := make(map[string]string)
	if info == nil {
		return headerOverride, nil
	}

	headerOverrideSource := relaycommon.GetEffectiveHeaderOverride(info)

	passAll := false
	var passthroughRegex []*regexp.Regexp
	if !info.IsChannelTest {
		for k := range headerOverrideSource {
			key := strings.TrimSpace(strings.ToLower(k))
			if key == "" {
				continue
			}
			if key == headerPassthroughAllKey {
				passAll = true
				continue
			}

			var pattern string
			switch {
			case strings.HasPrefix(key, headerPassthroughRegexPrefix):
				pattern = strings.TrimSpace(key[len(headerPassthroughRegexPrefix):])
			case strings.HasPrefix(key, headerPassthroughRegexPrefixV2):
				pattern = strings.TrimSpace(key[len(headerPassthroughRegexPrefixV2):])
			default:
				continue
			}

			if pattern == "" {
				return nil, types.NewError(fmt.Errorf("header passthrough regex pattern is empty: %q", k), types.ErrorCodeChannelHeaderOverrideInvalid)
			}
			compiled, err := getHeaderPassthroughRegex(pattern)
			if err != nil {
				return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
			}
			passthroughRegex = append(passthroughRegex, compiled)
		}
	}

	if passAll || len(passthroughRegex) > 0 {
		if c == nil || c.Request == nil {
			return nil, types.NewError(fmt.Errorf("missing request context for header passthrough"), types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		for name := range c.Request.Header {
			if shouldSkipPassthroughHeader(name) {
				continue
			}
			if !passAll {
				matched := false
				for _, re := range passthroughRegex {
					if re.MatchString(name) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			value := strings.TrimSpace(c.Request.Header.Get(name))
			if value == "" {
				continue
			}
			headerOverride[strings.ToLower(strings.TrimSpace(name))] = value
		}
	}

	for k, v := range headerOverrideSource {
		if isHeaderPassthroughRuleKey(k) {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(k))
		if key == "" {
			continue
		}

		str, ok := v.(string)
		if !ok {
			return nil, types.NewError(nil, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		if info.IsChannelTest && strings.HasPrefix(strings.TrimSpace(str), clientHeaderPlaceholderPrefix) {
			continue
		}

		value, include, err := applyHeaderOverridePlaceholders(str, c, info.ApiKey)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		if !include {
			continue
		}

		headerOverride[key] = value
	}
	return headerOverride, nil
}

// ResolveHeaderOverride returns the effective upstream header overrides for the request.
func ResolveHeaderOverride(info *relaycommon.RelayInfo, c *gin.Context) (map[string]string, error) {
	return resolveHeaderOverride(info, c)
}

// ApplyHeaderOverrideToRequest writes resolved header overrides onto the outbound request.
func ApplyHeaderOverrideToRequest(req *http.Request, headerOverride map[string]string) {
	applyHeaderOverrideToRequest(req, headerOverride)
}

func applyHeaderOverrideToRequest(req *http.Request, headerOverride map[string]string) {
	if req == nil {
		return
	}
	for key, value := range headerOverride {
		req.Header.Set(key, value)
		if strings.EqualFold(key, "Host") {
			req.Host = value
		}
	}
}
