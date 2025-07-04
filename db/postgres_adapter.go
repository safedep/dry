package db

import (
	"fmt"
	"time"

	"github.com/safedep/dry/log"
	"github.com/safedep/dry/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
	"gorm.io/plugin/prometheus"
)

// GORM internally uses pgx as the default driver for PostgreSQL. The DSN
// format can be anything supported by pgx. For example:
// postgres://${user}:${password}@${host}/${db}?sslmode=verify-none
type PostgreSqlAdapterConfig struct {
	DSN string

	// A name to report as part of the tracer plugin
	TracingDBName string

	// Translate errors to gorms internal error types
	TranslateError bool

	EnableTracing bool
	EnableMetrics bool

	// This is an optional pointer to the SqlAdapterConfig struct.
	// If not supplied, we will use the defaultSqlAdapterConfig.
	SqlAdapterConfig *SqlAdapterConfig
}

type PostgreSqlAdapter struct {
	*baseSqlAdapter

	db     *gorm.DB
	config PostgreSqlAdapterConfig
}

func NewPostgreSqlAdapter(config PostgreSqlAdapterConfig) (SqlDataAdapter, error) {
	dsn := config.DSN
	if dsn == "" {
		dsnFromEnv, err := DatabaseURL()
		if err != nil {
			return nil, fmt.Errorf("failed to get database URL from env: %w", err)
		}

		log.Debugf("Connecting to PostgreSQL database with DSN from env")
		dsn = dsnFromEnv
	} else {
		log.Debugf("Connecting to PostgreSQL database with DSN from config")
	}

	var db *gorm.DB
	var err error

	retry.InvokeWithRetry(retry.RetryConfig{
		Count: 30,
		Sleep: 1 * time.Second,
	}, func(arg retry.RetryFuncArg) error {
		// https://gorm.io/docs/connecting_to_the_database.html#PostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			TranslateError: config.TranslateError,
		})
		if err != nil {
			log.Debugf("[%d/%d] Failed to connect to PostgreSQL server: %v",
				arg.Current, arg.Total, err)
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	log.Debugf("PostgreSQL database connection established")

	if config.EnableTracing {
		if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem(config.TracingDBName))); err != nil {
			return nil, err
		}
	}

	if config.EnableMetrics {
		metricsMiddleware := prometheus.New(prometheus.Config{
			DBName:          config.TracingDBName,
			RefreshInterval: 15,
		})

		if err := db.Use(metricsMiddleware); err != nil {
			return nil, err
		}
	}

	baseSqlAdapter := &baseSqlAdapter{
		db:     db,
		config: config.SqlAdapterConfig,
	}

	err = baseSqlAdapter.SetupConnectionPool()
	if err != nil {
		return nil, err
	}

	postgreSqlAdapter := &PostgreSqlAdapter{db: db, config: config, baseSqlAdapter: baseSqlAdapter}

	err = postgreSqlAdapter.Ping()
	return postgreSqlAdapter, nil
}
