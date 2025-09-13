package obs

import (
	"os"

	gcpprof "cloud.google.com/go/profiler"
	"github.com/safedep/dry/log"
)

const (
	envKeyProfilerEnabled  = "APP_PROFILER_ENABLED"
	envKeyProfilerProvider = "APP_PROFILER_PROVIDER"
)

// InitProfiler initializes the profiler for performance monitoring.
// This function will panic and fail fast if the profiler cannot be started.
// Since profiling and data collection is a platform concern, this is a factory
// function and leverage environment variables to decide which profiler to start.
// Also profiling is performance sensitive, hence profiler will be started ONLY
// if the environment variable is set explicitly to start the profiler.
func InitProfiler() {
	if !isProfilerEnabled() {
		return
	}

	provider := os.Getenv(envKeyProfilerProvider)
	switch provider {
	case "google-cloud":
		initGoogleCloudProfiler()
	// Other profilers can go here. Ideally we don't need pprof because
	// pprof profiles can be generated output of the box using go test tool.
	// https://pkg.go.dev/runtime/pprof
	default:
		log.Warnf("profiler provider %q is not supported, skipping profiler initialization", provider)
	}
}

func initGoogleCloudProfiler() {
	// This will work out of the box in GCP because the profiler by default determines
	// GCP project, zone and other details from the metadata service.
	if err := gcpprof.Start(gcpprof.Config{
		Service:        AppServiceName(os.Getenv(tracerServiceNameEnvKey)),
		ServiceVersion: AppServiceEnv(os.Getenv(tracerServiceEnvEnvKey)),
	}); err != nil {
		panic("failed to start Google Cloud Profiler: " + err.Error())
	}
}

func isProfilerEnabled() bool {
	// The env value must be set to "true" explicitly to enable profiler.
	// We don't care about true'ish alternatives
	if s, ok := os.LookupEnv(envKeyProfilerEnabled); ok && (s == "true") {
		return true
	}

	return false
}
