package httpctx

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
)

// SetContextKey stores a typed context key on the current request context.
func SetContextKey(c *gin.Context, key constant.ContextKey, value any) {
	c.Set(string(key), value)
}

// GetContextKey returns the raw value stored under the typed context key.
func GetContextKey(c *gin.Context, key constant.ContextKey) (any, bool) {
	return c.Get(string(key))
}

// GetContextKeyString reads a string value stored under the typed context key.
func GetContextKeyString(c *gin.Context, key constant.ContextKey) string {
	return c.GetString(string(key))
}

// GetContextKeyInt reads an int value stored under the typed context key.
func GetContextKeyInt(c *gin.Context, key constant.ContextKey) int {
	return c.GetInt(string(key))
}

// GetContextKeyBool reads a bool value stored under the typed context key.
func GetContextKeyBool(c *gin.Context, key constant.ContextKey) bool {
	return c.GetBool(string(key))
}

// GetContextKeyStringSlice reads a string slice stored under the typed context key.
func GetContextKeyStringSlice(c *gin.Context, key constant.ContextKey) []string {
	return c.GetStringSlice(string(key))
}

// GetContextKeyStringMap reads a string map stored under the typed context key.
func GetContextKeyStringMap(c *gin.Context, key constant.ContextKey) map[string]any {
	return c.GetStringMap(string(key))
}

// GetContextKeyTime reads a time value stored under the typed context key.
func GetContextKeyTime(c *gin.Context, key constant.ContextKey) time.Time {
	return c.GetTime(string(key))
}

// GetContextKeyType reads a strongly typed value stored under the typed context key.
func GetContextKeyType[T any](c *gin.Context, key constant.ContextKey) (T, bool) {
	if value, ok := c.Get(string(key)); ok {
		if v, ok := value.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}
