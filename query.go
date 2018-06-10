package httputil

import "database/sql"

type Querier interface {
	QueryRow(query string, args ...interface{}) Scanner
	Query(query string, args ...interface{}) (Rower, error)
}

type Scanner interface {
	Scan(dest ...interface{}) error
}

type Rower interface {
	Scanner
	Next() bool
}

type Tx interface {
	XODB
	SqlxDB
	Commit() error
	Rollback() error
}

type Transaction interface {
	Begin() (tx Tx, err error)
	Commit(tx Tx) error
}

type XODB interface {
	Querier
	Exec(string, ...interface{}) (sql.Result, error)
}

type SqlxDB interface {
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
}

type DBInterface interface {
	XODB
	Transaction
	SqlxDB
}
