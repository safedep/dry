package db

import (
	"gorm.io/gorm"
)

// SqlDataAdapter represents a contract for implementing RDBMS data adapters
type SqlDataAdapter interface {
	GetDB() (*gorm.DB, error)
	Migrate(...interface{}) error
	Ping() error
}
