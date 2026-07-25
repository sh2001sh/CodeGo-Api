package textx

import "sync/atomic"

var debugEnabled atomic.Bool

// SetDebugEnabled configures whether text helpers should preserve full debug output.
func SetDebugEnabled(enabled bool) {
	debugEnabled.Store(enabled)
}

func isDebugEnabled() bool {
	return debugEnabled.Load()
}
