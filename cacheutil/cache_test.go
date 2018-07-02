package cacheutil

import (
	"errors"
	"fmt"
	"time"
)

type TestCacheStore struct{}

func (t TestCacheStore) Get(key string) ([]byte, error) {
	if key == "success" {
		return []byte("success"), nil
	}

	return nil, errors.New("nil")
}

func (t TestCacheStore) Set(key string, value interface{}, expiration time.Duration) {

}

func (t TestCacheStore) Del(keys ...string) {

}

var (
	cache CacheStore
)

func init() {
	cache = TestCacheStore{}
}

func ExampleClientCache_Get_ex1() {
	// ... Doing something and want to retrieve cache
	value, _ := cache.Get("success")
	fmt.Println(string(value))
	// Output: success
}

func ExampleClientCache_Get_ex2() {
	// ... Doing something and want to retrieve cache
	// But key item is not in cache
	_, err := cache.Get("fail")

	if err != nil {
		// If returns errors, then there was no key of given value
		// so do what you will with error

		fmt.Println(string(err.Error()))
		// Output: nil
	}
}
