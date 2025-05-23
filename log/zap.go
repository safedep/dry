package log

import (
	"os"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitZapLogger initializes a zap based logger
// and sets it as the default logger using SetGlobal
func InitZapLogger(name, env string) {
	skipStdoutLogger, err := strconv.ParseBool(os.Getenv(loggerKeySkipStdoutLogger))
	logStdout := true // default behavior
	if err == nil && skipStdoutLogger {
		logStdout = false // overriden behavior
	}

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

type zapLoggerWrapper struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
}

type zapLoggerConfig struct {
	logLevel  string // Log level (debug, info, warn, error)
	logFile   string // Log file path
	logStdout bool   // Whether to log to stdout
}

func newZapLogger(name, env string, config zapLoggerConfig) (Logger, error) {
	// Start with the default log level
	level := zap.NewAtomicLevelAt(zapcore.InfoLevel)

	// Override based on env configuration
	switch config.logLevel {
	case logLevelNameDebug:
		level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case logLevelNameWarn:
		level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case logLevelNameError:
		level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	}

	// Create zap cores that will be populated based on env configuration
	cores := []zapcore.Core{}

	if config.logStdout {
		// Our default console logger using development config
		developmentConfig := zap.NewDevelopmentEncoderConfig()
		developmentConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		consoleEncoder := zapcore.NewConsoleEncoder(developmentConfig)
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level))
	}

	// Add the file core with production config only when enabled
	if config.logFile != "" {
		productionConfig := zap.NewProductionEncoderConfig()
		productionConfig.TimeKey = "timestamp"
		productionConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		file := zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.logFile,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
		})

		fileEncoder := zapcore.NewJSONEncoder(productionConfig)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(file), level))
	}

	core := zapcore.NewTee(cores...)

	// We add a caller stack skip of 2 because the host app will be accessing the
	// zap logger through methods in utils, which in turn will invoke the global
	// logger implementation
	logger := zap.New(core, zap.AddCallerSkip(2))

	logger = logger.With(zap.String(loggerKeyServiceName, name))
	logger = logger.With(zap.String(loggerKeyServiceEnv, env))
	logger = logger.With(zap.String(loggerKeyLoggerType, "zap"))

	return &zapLoggerWrapper{
		logger:        logger,
		sugaredLogger: logger.Sugar(),
	}, nil
}

func (z *zapLoggerWrapper) Infof(msg string, args ...any) {
	z.sugaredLogger.Infof(msg, args...)
}

func (z *zapLoggerWrapper) Warnf(msg string, args ...any) {
	z.sugaredLogger.Warnf(msg, args...)
}

func (z *zapLoggerWrapper) Errorf(msg string, args ...any) {
	z.sugaredLogger.Errorf(msg, args...)
}

func (z *zapLoggerWrapper) Debugf(msg string, args ...any) {
	z.sugaredLogger.Debugf(msg, args...)
}

func (z *zapLoggerWrapper) Fatalf(msg string, args ...any) {
	z.sugaredLogger.Fatalf(msg, args...)
}

func (z *zapLoggerWrapper) With(args map[string]any) Logger {
	var fields []zap.Field
	for key, value := range args {
		fields = append(fields, zap.Any(key, value))
	}

	logger := z.logger.With(fields...).WithOptions(zap.AddCallerSkip(1))
	return &zapLoggerWrapper{
		logger:        logger,
		sugaredLogger: logger.Sugar(),
	}
}
