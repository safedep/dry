package grpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/keepalive"
)

func TestKeepaliveParams(t *testing.T) {
	want := keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}
	assert.Equal(t, want, keepaliveParams(), "pmg uses these values for the SafeDep API")
}
