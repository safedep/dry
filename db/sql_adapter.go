package db

import (
	"database/sql"
	"time"

	"github.com/safedep/dry/log"
	"golang.org/x/net/context"
	"gorm.io/gorm"
)

// SqlDataAdapter represents a contract for implementing RDBMS data adapters
type SqlDataAdapter interface {
	GetDB() (*gorm.DB, error)
	GetConn() (*sql.DB, error)
	Migrate(...interface{}) error
	Ping() error
}

type SqlAdapterConfig struct {
	MaxIdleConnections int
	MaxOpenConnections int
}

var defaultSqlAdapterConfig SqlAdapterConfig = SqlAdapterConfig{
	MaxIdleConnections: 5,
	MaxOpenConnections: 50,
}

func DefaultSqlAdapterConfig() SqlAdapterConfig {
	return defaultSqlAdapterConfig
}

type baseSqlAdapter struct {
	db     *gorm.DB
	config *SqlAdapterConfig
}

func (m *baseSqlAdapter) Config() *SqlAdapterConfig {
	if m.config != nil {
		return m.config
	}

	return &defaultSqlAdapterConfig
}

func (m *baseSqlAdapter) SetupConnectionPool() error {
	conn, err := m.GetConn()
	if err != nil {
		return err
	}

	log.Debugf("Setting up connection pool with max idle connections: %d, max open connections: %d",
		m.Config().MaxIdleConnections, m.Config().MaxOpenConnections)

	conn.SetMaxIdleConns(m.Config().MaxIdleConnections)
	conn.SetMaxOpenConns(m.Config().MaxOpenConnections)

	return nil
}

func (m *baseSqlAdapter) GetDB() (*gorm.DB, error) {
	return m.db, nil
}

func (m *baseSqlAdapter) GetConn() (*sql.DB, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}

	return db.DB()
}

func (m *baseSqlAdapter) Migrate(tables ...interface{}) error {
	return m.db.AutoMigrate(tables...)
}

func (m *baseSqlAdapter) Ping() error {
	sqlDB, err := m.GetConn()
	if err != nil {
		return err
	}

	ctx, cFunc := context.WithTimeout(context.Background(), 2*time.Second)
	defer cFunc()

	log.Debugf("Pinging database server")
	return sqlDB.PingContext(ctx)
}
