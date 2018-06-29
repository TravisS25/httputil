package cacheutil

import (
	"time"

	"github.com/go-redis/redis"
)

// CacheStore is interface used to get, set and delete cached values
// from structs that implement
type CacheStore interface {
	Get(key string) ([]byte, error)
	Set(key string, value interface{}, expiration time.Duration)
	Del(keys ...string)
}

// Default struct that implements the CacheStore interface
// The underlining implementation is based off of the
// "github.com/go-redis/redis" library
type ClientCache struct {
	*redis.Client
}

// Returns ClientCache pointer
func NewClientCache(client *redis.Client) *ClientCache {
	return &ClientCache{client}
}

// Gets
func (c *ClientCache) Get(key string) ([]byte, error) {
	return c.Client.Get(key).Bytes()
}

func (c *ClientCache) Set(key string, value interface{}, expiration time.Duration) {
	c.Client.Set(key, value, expiration)
}

func (c *ClientCache) Del(keys ...string) {
	c.Client.Del(keys...)
}
