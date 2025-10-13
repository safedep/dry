package sandbox

import (
	"os"
	"strconv"
	"testing"
)

func isExecutorEndToEndTestEnabled() bool {
	value := os.Getenv("EXECUTOR_END_TO_END_TESTS")
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return b
}

func getDockerSocketPath(t *testing.T) string {
	path := os.Getenv("TEST_DOCKER_EXECUTOR_SOCKET_PATH")
	if path == "" {
		t.Fatalf("TEST_DOCKER_EXECUTOR_SOCKET_PATH is not set")
	}

	return path
}
