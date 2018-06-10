package cacheutil

import (
	"time"

	"github.com/go-redis/redis"
)

type CacheStore interface {
	Get(key string) ([]byte, error)
	Set(key string, value interface{}, expiration time.Duration)
	Del(keys ...string)
}

type ClientCache struct {
	*redis.Client
}

func NewClientCache(client *redis.Client) *ClientCache {
	return &ClientCache{client}
}

func (c *ClientCache) Get(key string) ([]byte, error) {
	return c.Client.Get(key).Bytes()
}

func (c *ClientCache) Set(key string, value interface{}, expiration time.Duration) {
	c.Client.Set(key, value, expiration)
}

func (c *ClientCache) Del(keys ...string) {
	c.Client.Del(keys...)
}
