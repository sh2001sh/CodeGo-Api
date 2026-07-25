package app

import "github.com/gin-gonic/gin"

func ShouldSkipRetryAfterChannelAffinityFailure(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get("channel_affinity_skip_retry_on_failure")
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
