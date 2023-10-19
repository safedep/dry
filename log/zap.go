package log

import (
	"go.uber.org/zap"
)

// InitZapLogger initializes a zap based logger
// and sets it as the default logger using SetGlobal
func InitZapLogger(name string) {
	logger, err := newZapLogger(name)
	if err != nil {
		panic(err)
	}

	SetGlobal(logger)
}

type zapLoggerWrapper struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
}

func newZapLogger(name string) (Logger, error) {
	config := zap.NewDevelopmentConfig()

	// We add a caller stack skip of 2 because the host app will be accessing the
	// zap logger through methods in utils, which in turn will invoke the global
	// logger implementation
	logger, err := config.Build(zap.AddCallerSkip(2))

	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String(loggerKeyServiceName, name))
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
