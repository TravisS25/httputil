package cacheutil

import (
	"time"

	"github.com/go-redis/redis"
)

// CacheStore is interface used to get, set and delete cached values
// from structs that implement it
type CacheStore interface {
	Get(key string) ([]byte, error)
	Set(key string, value interface{}, expiration time.Duration)
	Del(keys ...string)
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
	return c.Client.Get(key).Bytes()
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

// // ClientCache is default struct that implements the CacheStore interface
// // ClientCache is just a wrapper to be able to implement CacheStore
// type ClientCache struct {
// 	CacheStore
// }

// // NewClientCache returns pointer of ClientCache
// func NewClientCache(cacheStore CacheStore) *ClientCache {
// 	return &ClientCache{cacheStore}
// }

// // Get gets value based on key passed
// // Returns error if key does not exist
// func (c *ClientCache) Get(key string) ([]byte, error) {
// 	return c.CacheStore.Get(key)
// }

// // Set sets value in redis server based on key and value given
// // Expiration sets how long the cache will stay in the server
// // If 0, key/value will never be deleted
// func (c *ClientCache) Set(key string, value interface{}, expiration time.Duration) {
// 	c.CacheStore.Set(key, value, expiration)
// }

// // Del deletes given string array of keys from server if exists
// func (c *ClientCache) Del(keys ...string) {
// 	c.CacheStore.Del(keys...)
// }
