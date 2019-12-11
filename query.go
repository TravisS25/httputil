package httputil

/*
 This file is basically used to keep together all database type interfaces together.
 These interfaces were created for the sole purpose of being able to unit test
 common database tasks such a querying a row/rows, looping through rows,
 scanning each row for results, transactions etc.
*/

import (
	"database/sql"
)

// Querier is for querying rows from database
type Querier interface {
	QueryRow(query string, args ...interface{}) Scanner
	Query(query string, args ...interface{}) (Rower, error)
}

// Scanner will scan row returned from database
type Scanner interface {
	Scan(dest ...interface{}) error
}

// Rower loops through rows returns from database with
// abilty to scan each row
type Rower interface {
	Scanner
	Next() bool
	Columns() ([]string, error)
}

// Tx is for transaction related queries
type Tx interface {
	XODB
	SqlxDB
	Commit() error
	Rollback() error
}

// Transaction is for ability to create database transaction
type Transaction interface {
	Begin() (tx Tx, err error)
	Commit(tx Tx) error
}

type QueryTransaction interface {
	Transaction
	Querier
}

// XODB allows to query rows but also exec statement against database
type XODB interface {
	Querier
	Exec(string, ...interface{}) (sql.Result, error)
}

// SqlxDB uses the sqlx library methods Get and Select for ability to
// easily query results into structs
type SqlxDB interface {
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
}

type Entity interface {
	XODB
	SqlxDB
}

// DBInterface is the main interface that should be used in your
// request handler functions
type DBInterface interface {
	Entity
	Transaction
}

type DBInterfaceV2 interface {
	DBInterface
	Recover
}

// type Recover interface {
// 	RecoverError(err error) bool
// }

type Recover interface {
	RecoverError(err error) (DBInterfaceV2, error)
}

type RecoverQuerier interface {
	Querier
	Recover
}

type FormSelection struct {
	Text  interface{} `json:"text"`
	Value interface{} `json:"value"`
}
