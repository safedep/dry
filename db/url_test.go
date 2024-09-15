package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseURL(t *testing.T) {
	t.Run("when set", func(t *testing.T) {
		t.Setenv(databaseDSNEnvKey, "postgresql://user:@localhost:5432/dbname")

		got, err := DatabaseURL()
		assert.NoError(t, err)
		assert.Equal(t, "postgresql://user:@localhost:5432/dbname", got)
	})

	t.Run("when not set", func(t *testing.T) {
		got, err := DatabaseURL()
		assert.ErrorContains(t, err, errDatabaseDSNNotSet.Error())
		assert.Empty(t, got)
	})
}

func TestMigrationDatabaseURL(t *testing.T) {
	cases := []struct {
		name            string
		dbEnvVal        string
		migrationEnvVal string
		want            string
		wantErr         error
	}{
		{
			name:            "database DSN environment is set",
			dbEnvVal:        "postgresql://user:@localhost:5432/dbname",
			migrationEnvVal: "",
			want:            "postgresql://user:@localhost:5432/dbname",
			wantErr:         nil,
		},
		{
			name:            "migration DSN environment is set",
			dbEnvVal:        "",
			migrationEnvVal: "postgresql://user:@localhost:5432/dbname",
			want:            "postgresql://user:@localhost:5432/dbname",
			wantErr:         nil,
		},
		{
			name:            "both DSN environments are set",
			dbEnvVal:        "postgresql://user:@localhost:5432/dbname1",
			migrationEnvVal: "postgresql://user:@localhost:5432/dbname2",
			want:            "postgresql://user:@localhost:5432/dbname2",
			wantErr:         nil,
		},
		{
			name:            "none are set",
			dbEnvVal:        "",
			migrationEnvVal: "",
			want:            "",
			wantErr:         errDatabaseDSNEmpty,
		},
		{
			name:            "invalid database DSN",
			dbEnvVal:        "invalid",
			migrationEnvVal: "",
			want:            "",
			wantErr:         errDatabaseSchemeInvalid,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(databaseDSNEnvKey, test.dbEnvVal)
			t.Setenv(databaseMigrationDSNEnvKey, test.migrationEnvVal)

			got, err := DatabaseMigrationURL()
			if test.wantErr != nil {
				assert.ErrorContains(t, err, test.wantErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.want, got)
			}
		})
	}
}
