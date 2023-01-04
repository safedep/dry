package cache

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/safedep/dry/log"
)

// Type for cache data
type CacheData []byte

// Represent an unique cache data identifier
// Multiple attributes are used to help in indexing / partitioning
type CacheKey struct {
	Source string
	Type   string
	Id     string
}

// Defines a type for a cacheable function (closure)
type CachableFunc[T any] func() (T, error)

// Internally maintained default caching adapter
var globalCachingAdapter Cache

// Cache define the contract for implementing caches
type Cache interface {
	Put(key *CacheKey, data *CacheData, ttl time.Duration) error
	Get(key *CacheKey) (*CacheData, error)
}

func init() {
	Disable()
}

func setGlobalCachingAdapter(a Cache) {
	globalCachingAdapter = a
}

func Disable() {
	setGlobalCachingAdapter(newNoCache())
}

func EnableWith(adapter Cache) {
	setGlobalCachingAdapter(adapter)
}

// Through define the devex sugar - Read through cache
func Through[T any](key *CacheKey, ttl time.Duration, fun CachableFunc[T]) (T, error) {
	system := globalCachingAdapter
	if system == nil {
		panic("default cache adapter is not set")
	}

	var empty T
	data, err := system.Get(key)
	if err != nil {
		// Cache lookup failed, invoke actual function
		realData, err := fun()
		if err != nil {
			return empty, err
		}

		// Cache output from actual function - Must not fail original path
		serializedData, err := JsonSerialize(realData)
		if err != nil {
			log.Warnf("Cache: Failed to serialize type:%T err:%v",
				realData, err)
			return realData, nil
		}

		err = system.Put(key, &serializedData, ttl)
		if err != nil {
			log.Warnf("Cache: Failed to put due to: %v", err)
		}

		return realData, nil
	} else {
		// Cache lookup is successful
		// Adapter bug in case of NPE on data
		return JsonDeserialize[T](*data)
	}
}

func JsonSerialize[T any](data T) (CacheData, error) {
	serialized, err := json.Marshal(data)
	if err != nil {
		return CacheData{}, err
	}

	return CacheData(serialized), nil
}

func JsonDeserialize[T any](data CacheData) (T, error) {
	var deserialized T
	err := json.NewDecoder(bytes.NewReader([]byte(data))).Decode(&deserialized)

	return deserialized, err
}
