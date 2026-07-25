package textx

import (
	"regexp"
	"strings"
)

var (
	upstreamQuotaLeakPattern   = regexp.MustCompile(`(?i)(status_code\s*=\s*403.*(?:预扣费额度失败|用户剩余额度|需要预扣费额度)|(?:预扣费额度失败|用户剩余额度|需要预扣费额度).*(?:request id\s*:|status_code\s*=)|用户额度不足[^\n]*(?:剩余额度|余额)[^\n]*(?:request id\s*:|status_code\s*=)|\binsufficient[\s_]+(?:quota|balance)\b|pre-?consume.*quota.*(?:request id\s*:|status_code\s*=))`)
	upstreamUnavailablePattern = regexp.MustCompile(`(?i)(\bno\s+available\s+(?:channel|route|provider|model)\b|\bno\s+(?:enabled|valid)\s+(?:channel|route|provider)\b|\b(?:model|service|channel|provider|upstream)\b[^\n]{0,80}\b(?:unavailable|not available|not found|not exist|not supported|temporarily unavailable)\b|(?:没有|无)可用(?:的)?(?:渠道|路由|供应商|模型)|(?:渠道|路由|模型|服务).{0,40}(?:不可用|不存在|不支持))`)
)

const UpstreamQuotaGenericMessage = "当前模型服务暂不可用，请稍后重试"

// SanitizeUpstreamProviderErrorMessage normalizes upstream capacity and quota errors.
func SanitizeUpstreamProviderErrorMessage(message string) string {
	if message == "" {
		return ""
	}
	lowerMessage := strings.ToLower(message)
	if strings.Contains(lowerMessage, "status_code=429") || strings.Contains(lowerMessage, "cooling down via provider") {
		return "status_code=429"
	}
	if IsUpstreamProviderUnavailableMessage(message) {
		return UpstreamQuotaGenericMessage
	}
	return message
}

// IsUpstreamProviderUnavailableMessage reports whether text exposes provider capacity details.
func IsUpstreamProviderUnavailableMessage(message string) bool {
	if message == "" {
		return false
	}
	return upstreamQuotaLeakPattern.MatchString(message) || upstreamUnavailablePattern.MatchString(message)
}

// SanitizeUpstreamQuotaErrorMessage is kept for callers that only need the legacy name.
func SanitizeUpstreamQuotaErrorMessage(message string) string {
	return SanitizeUpstreamProviderErrorMessage(message)
}

// IsUpstreamQuotaLeakMessage reports whether the message leaks upstream quota or balance details.
func IsUpstreamQuotaLeakMessage(message string) bool {
	if message == "" {
		return false
	}
	return upstreamQuotaLeakPattern.MatchString(message)
}
