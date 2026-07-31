package db

import (
	"context"
	"errors"
	"time"

	"github.com/safedep/dry/log"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogConfig configures the log-backed GORM query logger.
type GormLogConfig struct {
	// Level is one of "silent", "error", "warn", "info". Defaults to "warn".
	Level string

	// SlowThreshold logs queries slower than this as slow queries. Zero disables it.
	SlowThreshold time.Duration

	// IgnoreRecordNotFound suppresses gorm.ErrRecordNotFound from error logs.
	IgnoreRecordNotFound bool
}

// DefaultGormLogConfig returns production defaults: warn level, record-not-found ignored.
func DefaultGormLogConfig() GormLogConfig {
	return GormLogConfig{
		Level:                "warn",
		SlowThreshold:        200 * time.Millisecond,
		IgnoreRecordNotFound: true,
	}
}

// resolveGormLogger builds a logger from an optional config, defaulting to DefaultGormLogConfig.
func resolveGormLogger(config *GormLogConfig) gormlogger.Interface {
	cfg := DefaultGormLogConfig()
	if config != nil {
		cfg = *config
	}

	return NewGormLogger(nil, cfg)
}

func parseGormLogLevel(level string) gormlogger.LogLevel {
	switch level {
	case "silent", "Silent", "SILENT":
		return gormlogger.Silent
	case "error", "Error", "ERROR":
		return gormlogger.Error
	case "info", "Info", "INFO":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

// globalLogAdapter routes to the log package globals so a nil logger uses the app's global logger.
type globalLogAdapter struct{}

func (globalLogAdapter) Infof(msg string, args ...any)  { log.Infof(msg, args...) }
func (globalLogAdapter) Warnf(msg string, args ...any)  { log.Warnf(msg, args...) }
func (globalLogAdapter) Errorf(msg string, args ...any) { log.Errorf(msg, args...) }
func (globalLogAdapter) Debugf(msg string, args ...any) { log.Debugf(msg, args...) }
func (globalLogAdapter) Fatalf(msg string, args ...any) { log.Fatalf(msg, args...) }
func (globalLogAdapter) With(args map[string]any) log.Logger {
	return log.With(args)
}

// gormLogger implements gorm.io/gorm/logger.Interface on top of log.Logger.
type gormLogger struct {
	logger        log.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
	ignoreRNF     bool
}

// NewGormLogger builds a GORM logger backed by log.Logger. A nil logger uses the global.
func NewGormLogger(logger log.Logger, config GormLogConfig) gormlogger.Interface {
	if logger == nil {
		logger = globalLogAdapter{}
	}

	return &gormLogger{
		logger:        logger,
		level:         parseGormLogLevel(config.Level),
		slowThreshold: config.SlowThreshold,
		ignoreRNF:     config.IgnoreRecordNotFound,
	}
}

func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

func (l *gormLogger) Info(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Info {
		l.logger.Infof(msg, data...)
	}
}

func (l *gormLogger) Warn(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Warn {
		l.logger.Warnf(msg, data...)
	}
}

func (l *gormLogger) Error(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Error {
		l.logger.Errorf(msg, data...)
	}
}

func (l *gormLogger) Trace(_ context.Context, begin time.Time,
	fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6

	switch {
	case err != nil && l.level >= gormlogger.Error &&
		(!errors.Is(err, gormlogger.ErrRecordNotFound) || !l.ignoreRNF):
		sql, rows := fc()
		l.logger.With(map[string]any{
			"sql":        sql,
			"rows":       rows,
			"elapsed_ms": elapsedMs,
			"error":      err.Error(),
		}).Errorf("gorm: query error")

	case l.slowThreshold != 0 && elapsed > l.slowThreshold && l.level >= gormlogger.Warn:
		sql, rows := fc()
		l.logger.With(map[string]any{
			"sql":          sql,
			"rows":         rows,
			"elapsed_ms":   elapsedMs,
			"threshold_ms": float64(l.slowThreshold.Nanoseconds()) / 1e6,
		}).Warnf("gorm: slow query")

	case l.level >= gormlogger.Info:
		sql, rows := fc()
		l.logger.With(map[string]any{
			"sql":        sql,
			"rows":       rows,
			"elapsed_ms": elapsedMs,
		}).Infof("gorm: query")
	}
}
