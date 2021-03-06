package cacheutil

import (
	"errors"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/sessions"
	redistore "gopkg.in/boj/redistore.v1"
)

var (
	//ErrMockCache = errors.New("mockcache: testing")
	ErrCacheNil = errors.New("cacheutil: cache is nil")
)

// type MockCache struct {
// 	GetFunc    func(key string) ([]byte, error)
// 	HasKeyFunc func(key string) (bool, error)
// }

// func (m MockCache) Get(key string) ([]byte, error) {
// 	if m.GetFunc == nil {
// 		return nil, errors.New("mockcache: testing")
// 	}

// 	return m.GetFunc(key)
// }
// func (m MockCache) Set(key string, value interface{}, expiration time.Duration) {}
// func (m MockCache) Del(keys ...string)                                          {}
// func (m MockCache) HasKey(key string) (bool, error) {
// 	if m.HasKeyFunc == nil {
// 		errors.New("mockcache: testing")
// 	}

// 	return m.HasKeyFunc(key)
// }

// CacheStore is interface used to get, set and delete cached values
// from structs that implement it
type CacheStore interface {
	Get(key string) ([]byte, error)
	Set(key string, value interface{}, expiration time.Duration)
	Del(keys ...string)
	HasKey(key string) (bool, error)
}

type SessionStore interface {
	sessions.Store
	Ping() (bool, error)
}

// ClientCache is default struct that implements the CacheStore interface
// The underlining implementation is based off of the
// "github.com/go-redis/redis" library
type ClientCache struct {
	*redis.Client
}

// NewClientCache returns pointer of ClientCache
func NewClientCache(client *redis.Client) *ClientCache {
	return &ClientCache{client}
}

// Get gets value based on key passed
// Returns error if key does not exist
func (c *ClientCache) Get(key string) ([]byte, error) {
	var resultsErr error

	results, err := c.Client.Get(key).Bytes()

	if err != nil {
		if err == redis.Nil {
			resultsErr = ErrCacheNil
		} else {
			resultsErr = err
		}
	}

	return results, resultsErr
}

// Set sets value in redis server based on key and value given
// Expiration sets how long the cache will stay in the server
// If 0, key/value will never be deleted
func (c *ClientCache) Set(key string, value interface{}, expiration time.Duration) {
	c.Client.Set(key, value, expiration)
}

// Del deletes given string array of keys from server if exists
func (c *ClientCache) Del(keys ...string) {
	c.Client.Del(keys...)
}

// HasKey takes key value and determines if that key is in cache
func (c *ClientCache) HasKey(key string) (bool, error) {
	_, err := c.Get(key)

	if err != nil {
		return false, err
	}

	return true, nil
}

type SessionConfig struct {
	SessionName string
	Keys        SessionKeys
}

type SessionKeys struct {
	UserKey string
}

type RedisStore struct {
	*redistore.RediStore
}

func NewRedisStore(store *redistore.RediStore) *RedisStore {
	return &RedisStore{
		RediStore: store,
	}
}

func (r *RedisStore) Ping() (bool, error) {
	conn := r.RediStore.Pool.Get()
	defer conn.Close()
	data, err := conn.Do("PING")
	if err != nil || data == nil {
		return false, err
	}
	return (data == "PONG"), nil
}

type CacheValidateConfig struct {
	Cache CacheStore
	Key   string
}

type FormSelectionConfig struct {
	TextColumn       string
	ValueColumn      string
	FormSelectionKey string
}

// CacheSetup is configuration struct used to setup caching database tables
// that generally do not insert/update often
//
// CacheSetup should be used in a map where the key value is the string name of
// the database table to cache and CacheSetup is the value to use for setting up cache
type CacheSetup struct {
	// StringVal should be the "string" representation of the database table
	StringVal string

	// CacheIDKey should be the key value you will store the table id in cache
	CacheIDKey string

	// CacheListKey should be the key value you will store the whole table in cache
	CacheListKey string

	OrderByColumn string

	FormSelectionConf *FormSelectionConfig
}
