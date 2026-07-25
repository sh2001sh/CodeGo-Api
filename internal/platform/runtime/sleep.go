package runtime

import "time"

// RandomSleep pauses for up to three seconds to reduce contention on hot code paths.
func RandomSleep() {
	time.Sleep(time.Duration(GetRandomInt(3000)) * time.Millisecond)
}
