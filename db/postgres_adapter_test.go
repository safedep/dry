package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUncachedConnConfig(t *testing.T) {
	cases := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "a URL DSN", dsn: "postgres://u:p@localhost:5432/db?sslmode=disable"},
		{name: "a key value DSN", dsn: "host=localhost port=5432 user=u dbname=db sslmode=disable"},
		{name: "a DSN that does not parse", dsn: "postgres://u:p@localhost:notaport/db", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := uncachedConnConfig(tc.dsn)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, pgx.QueryExecModeDescribeExec, cfg.DefaultQueryExecMode)
			assert.Equal(t, "localhost", cfg.Host)
		})
	}
}

func TestPostgresDialector(t *testing.T) {
	const dsn = "postgres://u:p@localhost:5432/db?sslmode=disable"

	cached, err := postgresDialector(dsn, false)
	require.NoError(t, err)
	assert.Equal(t, "postgres", cached.Name())

	uncached, err := postgresDialector(dsn, true)
	require.NoError(t, err)
	assert.Equal(t, "postgres", uncached.Name())

	_, err = postgresDialector("postgres://u:p@localhost:notaport/db", true)
	assert.Error(t, err)
}
