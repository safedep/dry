package cache

import (
	"fmt"
	"time"
)

type noCache struct{}

func newNoCache() Cache {
	return &noCache{}
}

func (c *noCache) Put(key *CacheKey, data *CacheData, ttl time.Duration) error {
	return fmt.Errorf("nocache: no put")
}

func (c *noCache) Get(key *CacheKey) (*CacheData, error) {
	return nil, fmt.Errorf("nocache: no get")
}
