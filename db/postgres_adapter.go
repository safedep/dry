package db

import (
	"os"
	"time"

	"github.com/safedep/dry/log"
	"github.com/safedep/dry/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// GORM internally uses pgx as the default driver for PostgreSQL. The DSN
// format can be anything supported by pgx. For example:
// postgres://${user}:${password}@${host}/${db}?sslmode=verify-none
type PostgreSqlAdapterConfig struct {
	DSN string

	// A name to report as part of the tracer plugin
	TracingDBName string
}

type PostgreSqlAdapter struct {
	*baseSqlAdapter

	db     *gorm.DB
	config PostgreSqlAdapterConfig
}

func NewPostgreSqlAdapter(config PostgreSqlAdapterConfig) (SqlDataAdapter, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = config.DSN
		log.Debugf("Connecting to PostgreSQL database with DSN from config")
	} else {
		log.Debugf("Connecting to PostgreSQL database with DSN from env")
	}

	var db *gorm.DB
	var err error

	retry.InvokeWithRetry(retry.RetryConfig{
		Count: 30,
		Sleep: 1 * time.Second,
	}, func(arg retry.RetryFuncArg) error {
		// https://gorm.io/docs/connecting_to_the_database.html#PostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

	if err := db.Use(tracing.NewPlugin(tracing.WithDBName(config.TracingDBName))); err != nil {
		return nil, err
	}

	baseSqlAdapter := &baseSqlAdapter{db}
	postgreSqlAdapter := &PostgreSqlAdapter{db: db, config: config, baseSqlAdapter: baseSqlAdapter}

	err = postgreSqlAdapter.Ping()
	return postgreSqlAdapter, nil
}
