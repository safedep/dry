package db

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

const (
	databaseDSNEnvKey          = "DATABASE_URL"
	databaseTestDSNEnvKey      = "DATABASE_TEST_URL"
	databaseMigrationDSNEnvKey = "SCHEMA_MIGRATION_DATABASE_URL"
)

var (
	errDatabaseDSNNotSet     = errors.New("database DSN environment variable not set")
	errDatabaseDSNEmpty      = errors.New("database DSN environment variable is empty")
	errDatabaseSchemeInvalid = errors.New("database DSN scheme is empty")
)

// DatabaseURL returns the DSN to connect to the database
// using a conventional environment variable.
func DatabaseURL() (string, error) {
	return getDSNFromEnv(databaseDSNEnvKey)
}

// DatabaseTestURL returns the DSN to connect to the test database
// using a conventional environment variable.
func DatabaseTestURL() (string, error) {
	return getDSNFromEnv(databaseTestDSNEnvKey)
}

// DatabaseMigrationURL returns the DSN to connect to the database
// for schema migration using a conventional environment variable.
// If the schema migration DSN is not set, it will fallback to the
// regular database DSN.
func DatabaseMigrationURL() (string, error) {
	dsn, err := getDSNFromEnv(databaseMigrationDSNEnvKey)
	if err != nil {
		return DatabaseURL()
	}

	return dsn, err
}

func getDSNFromEnv(key string) (string, error) {
	dsn, ok := os.LookupEnv(key)
	if !ok {
		return "", errDatabaseDSNNotSet
	}

	if dsn == "" {
		return "", errDatabaseDSNEmpty
	}

	parsedDSN, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("failed to parse database DSN: %w", err)
	}

	if parsedDSN.Scheme == "" {
		return "", errDatabaseSchemeInvalid
	}

	return dsn, nil
}
