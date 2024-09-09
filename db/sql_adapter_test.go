package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSqlAdapterConfig(t *testing.T) {
	config := DefaultSqlAdapterConfig()

	assert.Equal(t, 5, config.MaxIdleConnections)
	assert.Equal(t, 50, config.MaxOpenConnections)
}
