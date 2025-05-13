package log

import (
	"os"
	"strconv"
)

// InitCliLogger initializes a zap based logger for CLI apps
// and sets it as the default logger using SetGlobal.
// It is different from ZapLogger as it needs
func InitCliLogger(name, env string) {
	logStdout, _ := strconv.ParseBool(os.Getenv(loggerKeyCliStdout))

	logger, err := newZapLogger(name, env, zapLoggerConfig{
		logLevel:  os.Getenv(loggerKeyEnvLogLevel),
		logFile:   os.Getenv(loggerKeyEnvLogFileName),
		logStdout: logStdout,
	})
	if err != nil {
		panic(err)
	}

	SetGlobal(logger)
}
