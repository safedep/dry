package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewZapLogger(t *testing.T) {
	logger, err := newZapLogger("TestSvc", "test")

	assert.Nil(t, err)
	assert.NotNil(t, logger)

	zapLogger, typeCheck := logger.(*zapLoggerWrapper)
	assert.True(t, typeCheck)

	assert.NotNil(t, zapLogger.logger)
	assert.NotNil(t, zapLogger.sugaredLogger)
}

func TestProductionLoggerToFile(t *testing.T) {
	t.Run("ProductionLoggerToFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.log")
		assert.NoError(t, err)

		path := tmpFile.Name()
		tmpFile.Close()

		// Remove the file created by os.CreateTemp
		os.Remove(path)

		// Make sure the file created by the test is removed
		defer os.Remove(path)

		t.Setenv(loggerKeyEnvLogFileName, path)
		t.Setenv(loggerKeyEnvLogLevel, logLevelNameDebug)

		l, err := newZapLogger("TestSvc", "test")
		assert.NoError(t, err)

		l.Infof("Test info log message")
		l.Debugf("Test debug log message")

		z, ok := l.(*zapLoggerWrapper)
		assert.True(t, ok)

		z.logger.Sync()

		assert.FileExists(t, path)

		content, err := os.ReadFile(path)
		assert.NoError(t, err)

		assert.Contains(t, string(content), "Test info log message")
		assert.Contains(t, string(content), "Test debug log message")
	})
}
