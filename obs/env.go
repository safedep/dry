package obs

import (
	"os"

	"github.com/safedep/dry/utils"
)

func AppServiceName(d string) string {
	n := os.Getenv(tracerServiceNameEnvKey)
	if utils.IsEmptyString(n) {
		n = d
	}

	return n
}

func AppServiceEnv(d string) string {
	n := os.Getenv(tracerServiceEnvEnvKey)
	if utils.IsEmptyString(n) {
		n = d
	}

	return n
}
