package db

import (
	"time"

	"golang.org/x/net/context"
	"gorm.io/gorm"
)

// SqlDataAdapter represents a contract for implementing RDBMS data adapters
type SqlDataAdapter interface {
	GetDB() (*gorm.DB, error)
	Migrate(...interface{}) error
	Ping() error
}

type baseSqlAdapter struct {
	db *gorm.DB
}

func (m *baseSqlAdapter) GetDB() (*gorm.DB, error) {
	return m.db, nil
}

func (m *baseSqlAdapter) Migrate(tables ...interface{}) error {
	return m.db.AutoMigrate(tables...)
}

func (m *baseSqlAdapter) Ping() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}

	ctx, cFunc := context.WithTimeout(context.Background(), 2*time.Second)
	defer cFunc()

	return sqlDB.PingContext(ctx)
}
