package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
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
	cases := []struct {
		name                  string
		disableStatementCache bool
		wantOwnConn           bool
	}{
		{name: "the default lets the gorm driver open the pool"},
		{name: "no statement cache opens the pool through pgx", disableStatementCache: true, wantOwnConn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := postgresDialector(dsn, tc.disableStatementCache)
			require.NoError(t, err)
			dialector, ok := d.(*postgres.Dialector)
			require.True(t, ok)
			assert.Equal(t, tc.wantOwnConn, dialector.Conn != nil)
		})
	}
}

func TestTimeZoneOptions(t *testing.T) {
	cases := []struct {
		name        string
		dsn         string
		wantZone    string
		wantOptions int
		wantErr     bool
	}{
		{name: "a DSN with no time zone", dsn: "postgres://u:p@localhost:5432/db?sslmode=disable"},
		{
			name: "a URL DSN with a time zone", dsn: "postgres://u:p@localhost:5432/db?TimeZone=Asia/Kolkata",
			wantZone: "Asia/Kolkata", wantOptions: 1,
		},
		{
			name: "a key value DSN with a time zone", dsn: "host=localhost user=u dbname=db timezone=Europe/Berlin",
			wantZone: "Europe/Berlin", wantOptions: 1,
		},
		{name: "an unknown time zone", dsn: "postgres://u:p@localhost:5432/db?TimeZone=Mars/Base", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := uncachedConnConfig(tc.dsn)
			require.NoError(t, err)
			options, err := timeZoneOptions(tc.dsn, cfg)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, options, tc.wantOptions)
			if tc.wantZone != "" {
				assert.Equal(t, tc.wantZone, cfg.RuntimeParams["timezone"])
			}
		})
	}
}
