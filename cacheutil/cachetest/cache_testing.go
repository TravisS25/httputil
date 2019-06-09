package cachetest

import (
	"errors"
	"time"
)

var (
	ErrMockCache = errors.New("mockcache: testing")
)

type MockCache struct {
	GetFunc    func(key string) ([]byte, error)
	HasKeyFunc func(key string) (bool, error)
}

func (m *MockCache) Get(key string) ([]byte, error) {
	if m.GetFunc == nil {
		return nil, errors.New("mockcache: testing")
	}

	return m.GetFunc(key)
}
func (m *MockCache) Set(key string, value interface{}, expiration time.Duration) {}
func (m *MockCache) Del(keys ...string)                                          {}
func (m *MockCache) HasKey(key string) (bool, error) {
	if m.HasKeyFunc == nil {
		errors.New("mockcache: testing")
	}

	return m.HasKeyFunc(key)
}
