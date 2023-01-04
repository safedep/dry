package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/safedep/dry/log"
)

type memoryCache struct {
	store sync.Map
}

// NewUnsafeMemoryCache returns an unbounded caching implementation not
// suitable for real life use
func NewUnsafeMemoryCache() Cache {
	return &memoryCache{
		store: sync.Map{},
	}
}

func (c *memoryCache) Put(key *CacheKey, data *CacheData, ttl time.Duration) error {
	log.Debugf("Memory Cache: Storing %v", *key)
	c.store.Store(fmt.Sprintf("%s/%s/%s", key.Source, key.Type, key.Id), data)

	return nil
}

func (c *memoryCache) Get(key *CacheKey) (*CacheData, error) {
	log.Debugf("Memory Cache: Loading %v", *key)
	result, ok := c.store.Load(fmt.Sprintf("%s/%s/%s", key.Source, key.Type, key.Id))
	if !ok {
		return nil, fmt.Errorf("key not in cache")
	}

	return result.(*CacheData), nil
}
