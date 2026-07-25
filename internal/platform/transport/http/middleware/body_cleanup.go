package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

// BodyStorageCleanup 璇锋眰浣撳瓨鍌ㄦ竻鐞嗕腑闂翠欢
// 鍦ㄨ姹傚鐞嗗畬鎴愬悗鑷姩娓呯悊纾佺洏/鍐呭瓨缂撳瓨
func BodyStorageCleanup() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		platformhttpx.CleanupBodyStorage(c)
		filex.CleanupFileSources(c)
	}
}
