package cache

import (
	"errors"
	"strconv"
	"time"

	"github.com/safedep/dry/db"
	"gorm.io/gorm"
)

const (
	mysqlCacheTableName = "caches"
)

var (
	mysqlCacheErrExpired      = errors.New("cache entry expired")
	mysqlCacheErrNonExistent  = errors.New("cache key not found")
	mysqlCacheErrBadParams    = errors.New("bad params")
	mysqlCacheErrActiveExists = errors.New("cache entry exists and active")
)

type mysqlCache struct {
	mysqlAdapter db.SqlDataAdapter
}

// Cache table
type mysqlCacheEntry struct {
	gorm.Model

	// Composite index as cache lookup key
	Source string `gorm:"type:varchar(100);not null;uniqueIndex:lookup_idx;priority:1"`
	Type   string `gorm:"type:varchar(100);not null;uniqueIndex:lookup_idx;priority:2"`
	Key    string `gorm:"type:varchar(100);not null;uniqueIndex:lookup_idx;priority:3"`

	// Cache data
	Data []byte `gorm:"type:mediumblob;not null"`

	// Cache TTL (Should be now + ttl)
	ExpiresAt time.Time `gorm:"not null"`
}

// Override table name
func (mysqlCacheEntry) TableName() string {
	return mysqlCacheTableName
}

type MySqlCacheConfig struct {
	Host, Port, Username, Password, Database string

	MaxIdleTime        time.Duration
	MaxOpenConnections int
}

func NewMySqlCache(config MySqlCacheConfig) (Cache, error) {
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 60 * time.Second
	}

	if config.MaxOpenConnections == 0 {
		config.MaxOpenConnections = 10
	}

	port, err := strconv.ParseInt(config.Port, 0, 16)
	if err != nil {
		port = 3306
	}

	dbConfig := db.MySqlAdapterConfig{
		Host:     config.Host,
		Port:     int16(port),
		Username: config.Username,
		Password: config.Password,
		Database: config.Database,
	}

	conn, err := db.NewMySqlAdapter(dbConfig)
	if err != nil {
		return nil, err
	}

	gDB, _ := conn.GetDB()
	dbConn, err := gDB.DB()
	if err != nil {
		return nil, err
	}

	dbConn.SetConnMaxIdleTime(config.MaxIdleTime)
	dbConn.SetMaxOpenConns(config.MaxOpenConnections)

	return &mysqlCache{mysqlAdapter: conn}, nil
}

func (mcache *mysqlCache) Put(key *CacheKey, data *CacheData, ttl time.Duration) error {
	if (key == nil) || (data == nil) {
		return mysqlCacheErrBadParams
	}

	entry := mysqlCacheEntry{
		Source:    key.Source,
		Type:      key.Type,
		Key:       key.Id,
		ExpiresAt: time.Now().Add(ttl),
		Data:      []byte(*data),
	}

	return mcache.createEntry(&entry)
}

func (mcache *mysqlCache) Get(key *CacheKey) (*CacheData, error) {
	record, err := mcache.findByCacheKey(key)
	if err != nil {
		return nil, err
	}

	if record.ExpiresAt.Before(time.Now()) {
		_ = mcache.deleteEntry(&record)
		return nil, mysqlCacheErrExpired
	}

	data := CacheData(record.Data)
	return &data, nil
}

func (mcache *mysqlCache) findByCacheKey(key *CacheKey) (mysqlCacheEntry, error) {
	var record mysqlCacheEntry

	db, err := mcache.mysqlAdapter.GetDB()
	if err != nil {
		return record, err
	}

	// Avoid soft delete flag
	tx := db.Unscoped().Where(&mysqlCacheEntry{
		Source: key.Source,
		Type:   key.Type,
		Key:    key.Id,
	}).First(&record)

	if tx.Error != nil {
		return record, tx.Error
	}

	return record, nil
}

func (mcache *mysqlCache) createEntry(entry *mysqlCacheEntry) error {
	db, err := mcache.mysqlAdapter.GetDB()
	if err != nil {
		return err
	}

	tx := db.Create(entry)
	return tx.Error
}

func (mcache *mysqlCache) deleteEntry(entry *mysqlCacheEntry) error {
	db, err := mcache.mysqlAdapter.GetDB()
	if err != nil {
		return err
	}

	// Force hard delete
	tx := db.Unscoped().Delete(entry)
	return tx.Error
}
