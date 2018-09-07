package cacheutil

import (
	"fmt"
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

type CacheStoreV2 interface {
	CacheStore
	HasKey(keyString string, keyIDs ...string) bool
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

// HasKey takes keyString and any id identifiers and checks if the key exists
// in cache server
// Returns true if it does, false otherwise
func (c *ClientCache) HasKey(keyString string, keyIDs ...string) bool {
	key := fmt.Sprintf(keyString, keyIDs)
	_, err := c.Client.Get(key).Bytes()

	if err != nil {
		return false
	}

	return true
}

func ConcatenateCacheKey(keyString string, valueStrings ...string) string {
	return fmt.Sprintf(keyString, valueStrings)
}

// func SetCacheForIDs(cache CacheStore, values interface{}, variableName, keyString string) error {
// 	s := reflect.ValueOf(values)

// 	if s.Kind() != reflect.Slice {
// 		return errors.New("values parameter must be slice")
// 	}

// 	sliceValues := s.Interface().([]interface{})

// 	for i := 0; i < len(sliceValues); i++ {
// 		e := reflect.ValueOf(sliceValues[i])

// 		fmt.Printf("her we goooo: %s", e)

// 		for k := 0; i < e.NumField(); i++ {
// 			if e.Type().Field(k).Name == variableName {
// 				key := ConcatenateCacheKey(keyString, "1")
// 				cache.Set(key, e.String(), 0)
// 			}
// 		}
// 	}

// 	return nil
// }
