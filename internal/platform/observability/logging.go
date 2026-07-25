package observability

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhostinfo "github.com/sh2001sh/CodeGo-Api/internal/platform/hostinfo"
)

// LogWriterMu protects concurrent access to gin.DefaultWriter and gin.DefaultErrorWriter.
var LogWriterMu sync.RWMutex

// SysLog writes a system log line to the default stdout logger.
func SysLog(message string) {
	now := time.Now()
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultWriter, "[SYS] %v | %s \n", now.Format("2006/01/02 - 15:04:05"), message)
	LogWriterMu.RUnlock()
}

// SysError writes a system log line to the default stderr logger.
func SysError(message string) {
	now := time.Now()
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[SYS] %v | %s \n", now.Format("2006/01/02 - 15:04:05"), message)
	LogWriterMu.RUnlock()
}

// FatalLog writes a fatal log line and terminates the process.
func FatalLog(values ...any) {
	now := time.Now()
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", now.Format("2006/01/02 - 15:04:05"), values)
	LogWriterMu.RUnlock()
	os.Exit(1)
}

// LogStartupSuccess prints the startup banner and reachable endpoints.
func LogStartupSuccess(startTime time.Time, port string) {
	durationMs := time.Since(startTime).Milliseconds()
	networkIPs := platformhostinfo.GetNetworkIPs()

	LogWriterMu.RLock()
	defer LogWriterMu.RUnlock()

	fmt.Fprintf(gin.DefaultWriter, "\n")
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", platformconfig.SystemName, platformconfig.Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	if !platformhostinfo.IsRunningInContainer() {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	for _, ip := range networkIPs {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	fmt.Fprintf(gin.DefaultWriter, "\n")
}
