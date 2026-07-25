package runtime

import (
	"fmt"
	"time"
)

// GetTimestamp returns the current Unix timestamp in seconds.
func GetTimestamp() int64 {
	return time.Now().Unix()
}

// GetTimeString returns a UTC timestamp string with nanosecond suffix for request IDs.
func GetTimeString() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s%d", now.Format("20060102150405"), now.UnixNano()%1e9)
}
