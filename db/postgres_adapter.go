package db

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
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

	// GormLog configures query logging via the log package. Nil uses DefaultGormLogConfig.
	GormLog *GormLogConfig

	// DisableStatementCache stops pgx from caching a prepared statement for
	// each SQL text on each connection. The cache holds up to 512 statements
	// per connection. A caller whose SQL text varies with its input, such as an
	// IN list or a multi-row insert, fills it with large statements that it
	// never runs again, and the process memory grows until the cache is full.
	// With the cache off, pgx describes and runs each query without a cache.
	DisableStatementCache bool
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

	dialector, err := postgresDialector(dsn, config.DisableStatementCache)
	if err != nil {
		return nil, err
	}

	var db *gorm.DB

	retry.InvokeWithRetry(retry.RetryConfig{
		Count: 30,
		Sleep: 1 * time.Second,
	}, func(arg retry.RetryFuncArg) error {
		// https://gorm.io/docs/connecting_to_the_database.html#PostgreSQL
		db, err = gorm.Open(dialector, &gorm.Config{
			TranslateError: config.TranslateError,
			Logger:         resolveGormLogger(config.GormLog),
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

// postgresDialector returns the GORM dialector for dsn. With the statement
// cache off it opens the pool through pgx with a connection config of its own,
// because the gorm driver takes no pgx options.
func postgresDialector(dsn string, disableStatementCache bool) (gorm.Dialector, error) {
	if !disableStatementCache {
		return postgres.Open(dsn), nil
	}
	connConfig, err := uncachedConnConfig(dsn)
	if err != nil {
		return nil, err
	}
	options, err := timeZoneOptions(dsn, connConfig)
	if err != nil {
		return nil, err
	}
	return postgres.New(postgres.Config{Conn: stdlib.OpenDB(*connConfig, options...)}), nil
}

// dsnTimeZone is the pattern the gorm driver uses to find the time zone of a DSN.
var dsnTimeZone = regexp.MustCompile("(time_zone|TimeZone|timezone)=(.*?)($|&| )")

// timeZoneOptions does for a pool that dry opens what the gorm driver does for
// the pool it opens. It sets the session time zone of the DSN, and it scans a
// timestamp without time zone in that zone. Without it pgx reads such a value
// as UTC, and the uncached path returns a different instant than the default.
func timeZoneOptions(dsn string, connConfig *pgx.ConnConfig) ([]stdlib.OptionOpenDB, error) {
	match := dsnTimeZone.FindStringSubmatch(dsn)
	if len(match) <= 2 {
		return nil, nil
	}
	loc, err := time.LoadLocation(match[2])
	if err != nil {
		return nil, fmt.Errorf("failed to load the DSN time zone %q: %w", match[2], err)
	}
	connConfig.RuntimeParams["timezone"] = match[2]
	return []stdlib.OptionOpenDB{stdlib.OptionAfterConnect(func(_ context.Context, conn *pgx.Conn) error {
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "timestamp",
			OID:   pgtype.TimestampOID,
			Codec: &pgtype.TimestampCodec{ScanLocation: loc},
		})
		return nil
	})}, nil
}

// uncachedConnConfig parses dsn and makes pgx describe each query before it
// runs it, with no statement cache. Describe keeps the parameter types that the
// server reports, which the exec mode would infer from the Go values.
func uncachedConnConfig(dsn string) (*pgx.ConnConfig, error) {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PostgreSQL DSN: %w", err)
	}
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	return connConfig, nil
}
