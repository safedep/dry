package db

import (
	"fmt"
	"os"
	"time"

	"github.com/safedep/dry/log"
	"github.com/safedep/dry/retry"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gorm.io/plugin/opentelemetry/tracing"
)

type MySqlAdapter struct {
	*baseSqlAdapter

	db     *gorm.DB
	config MySqlAdapterConfig
}

type MySqlAdapterConfig struct {
	Host     string
	Port     int16
	Username string
	Password string
	Database string
}

func NewMySqlAdapter(config MySqlAdapterConfig) (SqlDataAdapter, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.Username, config.Password, config.Host, config.Port, config.Database)
		log.Debugf("Connecting to MySQL database with %s@%s:%d", config.Username, config.Host, config.Port)
	} else {
		log.Debugf("Connecting to MySQL database with DSN from env")
	}

	var db *gorm.DB
	var err error

	retry.InvokeWithRetry(retry.RetryConfig{
		Count: 30,
		Sleep: 1 * time.Second,
	}, func(arg retry.RetryFuncArg) error {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Debugf("[%d/%d] Failed to connect to MySQL server: %v",
				arg.Current, arg.Total, err)
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	if err := db.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		return nil, err
	}

	baseSqlAdapter := &baseSqlAdapter{db: db}
	mysqlAdapter := &MySqlAdapter{db: db, config: config, baseSqlAdapter: baseSqlAdapter}

	err = mysqlAdapter.Ping()
	return mysqlAdapter, err
}
