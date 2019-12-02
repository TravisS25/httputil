package dbtest

import (
	"database/sql"

	"github.com/TravisS25/httputil"
)

// --------------------------- TEST SUITES ------------------------------

type logTableReturn struct {
	ID        interface{}
	TimeStamp string
}

type PreTestConfig struct {
	LogTable     string
	TimeStampCol string
}

type PostTestConfig struct {
	LogTable     string
	DBTableCol   string
	DBIDCol      string
	TimeStampCol string
}

type MockDB struct {
	QueryRowFunc func(query string, args ...interface{}) httputil.Scanner
	QueryFunc    func(query string, args ...interface{}) (httputil.Rower, error)
	ExecFunc     func(string, ...interface{}) (sql.Result, error)

	BeginFunc  func() (tx httputil.Tx, err error)
	CommitFunc func(tx httputil.Tx) error

	GetFunc    func(dest interface{}, query string, args ...interface{}) error
	SelectFunc func(dest interface{}, query string, args ...interface{}) error

	RecoverErrorFunc func(err error) bool
}

func (m *MockDB) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return m.QueryRowFunc(query, args...)
}

func (m *MockDB) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return m.QueryFunc(query, args...)
}

func (m *MockDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.Exec(query, args...)
}

func (m *MockDB) Begin() (tx httputil.Tx, err error) {
	return m.BeginFunc()
}

func (m *MockDB) Commit(tx httputil.Tx) error {
	return m.CommitFunc(tx)
}

func (m *MockDB) Get(dest interface{}, query string, args ...interface{}) error {
	return m.GetFunc(dest, query, args...)
}

func (m *MockDB) Select(dest interface{}, query string, args ...interface{}) error {
	return m.SelectFunc(dest, query, args...)
}

func (m *MockDB) RecoverError(err error) bool {
	return m.RecoverErrorFunc(err)
}
