package sandbox

import (
	"os"
	"strconv"
	"testing"
)

func isSandboxEndToEndTestEnabled() bool {
	value := os.Getenv("SANDBOX_ENABLE_E2E_TEST")
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

func getDockerSocketPath(t *testing.T) string {
	path := os.Getenv("TEST_DOCKER_SANDBOX_SOCKET_PATH")
	if path == "" {
		t.Fatalf("TEST_DOCKER_SANDBOX_SOCKET_PATH is not set")
	}

	return path
}
