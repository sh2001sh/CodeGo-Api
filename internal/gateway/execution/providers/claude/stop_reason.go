package claude

import (
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
)

func stopReasonClaude2OpenAI(reason string) string {
	switch strings.ToLower(reason) {
	case "stop_sequence", "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "refusal":
		return constant.FinishReasonContentFilter
	default:
		return reason
	}
}
