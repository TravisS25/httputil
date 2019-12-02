package cachetest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

var (
	ErrMockCache = errors.New("cachetest: testing")
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

type MockSessionStore struct {
	GetFunc  func(r *http.Request, name string) (*sessions.Session, error)
	NewFunc  func(r *http.Request, name string) (*sessions.Session, error)
	SaveFunc func(r *http.Request, w http.ResponseWriter, s *sessions.Session) error
	PingFunc func() (bool, error)
}

func (m *MockSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return m.GetFunc(r, name)
}

func (m *MockSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return m.NewFunc(r, name)
}

func (m *MockSessionStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return m.SaveFunc(r, w, s)
}

func (m *MockSessionStore) Ping() (bool, error) {
	return m.PingFunc()
}

func NewMockSessionError(cause error, err string, isUsage, isDecode, isInternal bool) *MockSessionError {
	return &MockSessionError{
		cause:      cause,
		err:        err,
		isUsage:    isUsage,
		isDecode:   isDecode,
		isInternal: isInternal,
	}
}

type MockSessionError struct {
	err        string
	cause      error
	isUsage    bool
	isDecode   bool
	isInternal bool
}

func (m *MockSessionError) Error() string {
	return m.err
}

func (m *MockSessionError) IsUsage() bool {
	return m.isUsage
}

func (m *MockSessionError) IsDecode() bool {
	return m.isDecode
}

func (m *MockSessionError) IsInternal() bool {
	return m.isInternal
}

func (m *MockSessionError) Cause() error {
	return m.cause
}

func (m *MockSessionError) SetError(s string) {
	m.err = s
}

func (m *MockSessionError) SetUsage(isUsage bool) {
	m.isUsage = isUsage
}

func (m *MockSessionError) SetDecode(isDecode bool) {
	m.isDecode = isDecode
}

func (m *MockSessionError) SetInternal(isInternal bool) {
	m.isInternal = isInternal
}

func (m *MockSessionError) SetCause(cause error) {
	m.cause = cause
}
