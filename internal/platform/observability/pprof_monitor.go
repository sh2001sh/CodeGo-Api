package observability

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

var pprofMonitorOnce sync.Once

// StartPprofCPUMonitor starts a background CPU sampler that writes pprof captures on high usage.
func StartPprofCPUMonitor() {
	pprofMonitorOnce.Do(func() {
		go func() {
			for {
				percent, err := cpu.Percent(time.Second, false)
				if err != nil {
					log.Printf("pprof cpu monitor failed: %v", err)
					time.Sleep(30 * time.Second)
					continue
				}
				if len(percent) == 0 || percent[0] <= 80 {
					time.Sleep(30 * time.Second)
					continue
				}

				fmt.Println("cpu usage too high")
				if err := os.MkdirAll("./pprof", os.ModePerm); err != nil {
					log.Printf("failed to create pprof directory: %v", err)
					time.Sleep(30 * time.Second)
					continue
				}
				file, err := os.Create("./pprof/" + fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")))
				if err != nil {
					log.Printf("failed to create pprof file: %v", err)
					time.Sleep(30 * time.Second)
					continue
				}
				if err := pprof.StartCPUProfile(file); err != nil {
					_ = file.Close()
					log.Printf("failed to start cpu profile: %v", err)
					time.Sleep(30 * time.Second)
					continue
				}

				time.Sleep(10 * time.Second)
				pprof.StopCPUProfile()
				_ = file.Close()
				time.Sleep(30 * time.Second)
			}
		}()
	})
}
