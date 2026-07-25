package observability

import (
	"runtime"

	"github.com/grafana/pyroscope-go"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
)

// StartPyroscope configures and starts the optional Pyroscope profiler.
func StartPyroscope() error {
	pyroscopeURL := platformconfig.GetEnvOrDefaultString("PYROSCOPE_URL", "")
	if pyroscopeURL == "" {
		return nil
	}

	pyroscopeAppName := platformconfig.GetEnvOrDefaultString("PYROSCOPE_APP_NAME", "codego-api")
	pyroscopeBasicAuthUser := platformconfig.GetEnvOrDefaultString("PYROSCOPE_BASIC_AUTH_USER", "")
	pyroscopeBasicAuthPassword := platformconfig.GetEnvOrDefaultString("PYROSCOPE_BASIC_AUTH_PASSWORD", "")
	pyroscopeHostname := platformconfig.GetEnvOrDefaultString("HOSTNAME", "codego-api")

	mutexRate := platformconfig.GetEnvOrDefaultInt("PYROSCOPE_MUTEX_RATE", 5)
	blockRate := platformconfig.GetEnvOrDefaultInt("PYROSCOPE_BLOCK_RATE", 5)

	runtime.SetMutexProfileFraction(mutexRate)
	runtime.SetBlockProfileRate(blockRate)

	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: pyroscopeAppName,

		ServerAddress:     pyroscopeURL,
		BasicAuthUser:     pyroscopeBasicAuthUser,
		BasicAuthPassword: pyroscopeBasicAuthPassword,

		Logger: nil,

		Tags: map[string]string{"hostname": pyroscopeHostname},

		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	return err
}
