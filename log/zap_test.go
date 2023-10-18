package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewZapLogger(t *testing.T) {
	logger, err := newZapLogger("TestSvc")

	assert.Nil(t, err)
	assert.NotNil(t, logger)

	zapLogger, typeCheck := logger.(*zapLoggerWrapper)
	assert.True(t, typeCheck)

	assert.NotNil(t, zapLogger.logger)
	assert.NotNil(t, zapLogger.sugaredLogger)
}
