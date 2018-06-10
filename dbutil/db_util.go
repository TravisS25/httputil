package dbutil

import (
	"database/sql"
	"fmt"

	"github.com/TravisS25/httputil"
	"github.com/jmoiron/sqlx"
)

const (
	INSERT = "INSERT"
	UPDATE = "UPDATE"
	DELETE = "DELETE"
)

const (
	SSLDisable    = "disable"
	SSLRequire    = "require"
	SSLVerifyCA   = "verify-ca"
	SSLVerifyFull = "verify-full"
)

//--------------------------- TYPES --------------------------------

type DBConfig struct {
	User     string
	Password string
	Name     string
	Port     string
	SSLMode  string
}

type Count struct {
	Total int `json:"total"`
}

func NewCustomTx(tx *sqlx.Tx) *CustomTx {
	return &CustomTx{
		tx: tx,
	}
}

type CustomTx struct {
	tx *sqlx.Tx
}

func (c *CustomTx) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return c.tx.QueryRow(query, args...)
}

func (c *CustomTx) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return c.tx.Query(query, args...)
}

func (c *CustomTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.tx.Exec(query, args...)
}

func (c *CustomTx) Commit() error {
	return c.tx.Commit()
}

func (c *CustomTx) Rollback() error {
	return c.tx.Rollback()
}

func (c *CustomTx) Get(dest interface{}, query string, args ...interface{}) error {
	return c.tx.Get(dest, query, args...)
}

func (c *CustomTx) Select(dest interface{}, query string, args ...interface{}) error {
	return c.tx.Select(dest, query, args...)
}

type DB struct {
	*sqlx.DB
}

func (db *DB) Begin() (httputil.Tx, error) {
	tx, _ := db.DB.Beginx()
	return NewCustomTx(tx), nil
}

func (db *DB) Commit(tx httputil.Tx) error {
	return tx.Commit()
}

func (db *DB) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return db.DB.QueryRow(query, args...)
}

func (db *DB) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return db.DB.Query(query, args...)
}

//----------------------------- FUNCTIONS -------------------------------------

func NewDB(dbConfig DBConfig) (*DB, error) {
	if dbConfig.SSLMode == "" {
		dbConfig.SSLMode = SSLDisable
	}

	dbInfo := fmt.Sprintf(
		"user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Name,
		dbConfig.Port,
		dbConfig.SSLMode,
	)

	db, err := sqlx.Open("postgres", dbInfo)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func QueryCount(db httputil.SqlxDB, query string, args ...interface{}) (*Count, error) {
	var dest Count
	err := db.Get(&dest, query, args...)
	return &dest, err
}
