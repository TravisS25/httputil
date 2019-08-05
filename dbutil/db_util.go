package dbutil

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/TravisS25/httputil/confutil"

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

const (
	Postgres = "postgres"
	Mysql    = "mysql"
)

var (
	ErrEmptyConfigList = errors.New("dbutil: Can't have empty config list")
	ErrNoConnection    = errors.New("dbutil: Connection could not be established")
)

//------------------------ INTERFACES ---------------------------

// type CustomMarshalJSON interface {
// 	SetExclusionJSONFields(fields map[string]bool)
// }

//--------------------------- TYPES --------------------------------

// DBConfig is config struct used in conjunction with NewDB function
// Allows user to easily set configuration for database they want to
// connect to
// type DBConfig struct {
// 	Host     string
// 	User     string
// 	Password string
// 	DBName   string
// 	Port     string
// 	SSLMode  string
// }

// Count is used to retrieve from count queries
type Count struct {
	Total int `json:"total"`
}

// NewCustomTx return *CustomTx
// CustomTx extends off of the sql.Tx library
// but adds some functionality like the Select and
// Get functions that are wrappers for the sqlx.Select and
// sqlx.Get functions
func NewCustomTx(tx *sqlx.Tx) *CustomTx {
	return &CustomTx{
		tx: tx,
	}
}

// CustomTx is struct that extends off of sql.Tx
type CustomTx struct {
	tx *sqlx.Tx
}

// QueryRow is wrapper for sql.QueryRow with custom return of httputil.Scanner
func (c *CustomTx) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return c.tx.QueryRow(query, args...)
}

// Query is wrapper for sql.Query with custom return of httputil.Rower
func (c *CustomTx) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return c.tx.Query(query, args...)
}

// Exec is wrapper for sql.Exec
func (c *CustomTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.tx.Exec(query, args...)
}

// Commit is wrapper for sql.Tx.Commit
func (c *CustomTx) Commit() error {
	return c.tx.Commit()
}

// Rollback is wrapper for sql.Tx.Rollback
func (c *CustomTx) Rollback() error {
	return c.tx.Rollback()
}

// Get is wrapper for sqlx.Get
func (c *CustomTx) Get(dest interface{}, query string, args ...interface{}) error {
	return c.tx.Get(dest, query, args...)
}

// Select is wrapper for sqlx.Select
func (c *CustomTx) Select(dest interface{}, query string, args ...interface{}) error {
	return c.tx.Select(dest, query, args...)
}

// DB extends sqlx.DB with some extra functions
type DB struct {
	*sqlx.DB
	dbConfigList []confutil.Database
	dbType       string
	mu           sync.Mutex
}

// Begin is wrapper for sqlx.DB.Begin
func (db *DB) Begin() (httputil.Tx, error) {
	tx, _ := db.DB.Beginx()
	return NewCustomTx(tx), nil
}

// Commit is wrapper for sqlx.Tx.Commit
func (db *DB) Commit(tx httputil.Tx) error {
	return tx.Commit()
}

// QueryRow is wrapper for sqlx.DB.QueryRow
func (db *DB) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return db.DB.QueryRow(query, args...)
}

// Query is wrapper for sqlx.DB.Query
func (db *DB) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return db.DB.Query(query, args...)
}

// RecoverError will check if given err is not nil and if it is
// it will loop through dbConfigList, if any, and try to establish
// a new connection with a different database
//
// This function should be used if you have a distributed type database
// etc. CockroachDB and don't want any interruptions if a node goes down
//
// This function does not check what type of err is passed, just checks
// if err is nil or not so it's up to user to use appropriately; however
// we do a quick ping check just to make sure db is truely down
func (db *DB) RecoverError(err error) bool {
	if err != nil {
		db.mu.Lock()
		err = db.Ping()

		if err != nil {
			if len(db.dbConfigList) == 0 {
				db.mu.Unlock()
				return false
			}

			//db.mu.Lock()
			// db.validConn = false
			foundNewConnection := false

			for _, v := range db.dbConfigList {
				newDB, err := NewDB(v, db.dbType)

				if err == nil {
					db = newDB
					foundNewConnection = true
					//db.validConn = true
					break
				}
			}

			if !foundNewConnection {
				db.mu.Unlock()
				return false
			}

			// if !db.validConn {
			// 	db.mu.Unlock()
			// 	return false
			// }

			return true
		}

		db.mu.Unlock()
		return true
	}
	return true
}

//----------------------------- FUNCTIONS -------------------------------------

// NewDB is function that returns *DB with given DB config
// If db connection fails, returns error
func NewDB(dbConfig confutil.Database, dbType string) (*DB, error) {
	dbInfo := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbConfig.Host,
		dbConfig.User,
		dbConfig.Password,
		dbConfig.DBName,
		dbConfig.Port,
		dbConfig.SSLMode,
	)

	db, err := sqlx.Open(dbType, dbInfo)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{DB: db}, nil
}

func NewDBWithList(dbConfigList []confutil.Database, dbType string) (*DB, error) {
	if len(dbConfigList) == 0 {
		return nil, ErrEmptyConfigList
	}

	for _, v := range dbConfigList {
		newDB, err := NewDB(v, dbType)

		if err == nil {
			newDB.dbConfigList = dbConfigList
			newDB.dbType = dbType
			//newDB.validConn = true
			return newDB, nil
		}
	}

	return nil, ErrNoConnection
}

func dbError(w http.ResponseWriter, err error, db httputil.DBInterfaceV2) bool {
	if err != nil {
		confutil.CheckError(err, "")

		if db.RecoverError(err) {
			w.WriteHeader(http.StatusTemporaryRedirect)
			return true
		}

		return true
	}

	return false
}

// HasDBError will check if given err is not nil and if it is
// it will loop through dbConfigList, if any, and try to establish
// a new connection with a different database
//
// This function should be used if you have a distributed type database
// etc. CockroachDB and don't want any interruptions if a node goes down
//
// This function does not check what type of err is passed, just checks
// if err is nil or not so it's up to user to use appropriately; however
// we do a quick ping check just to make sure db is truely down
// func HasDBError(w http.ResponseWriter, db httputil.DBInterfaceV2, err error) bool {
// 	return dbError(w, err, db)
// }

func HasDBError(w http.ResponseWriter, err error, db httputil.DBInterfaceV2) bool {
	return dbError(w, err, db)
}

func HasQueryOrDBError(w http.ResponseWriter, err error, db httputil.DBInterfaceV2, notFound string) bool {
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(notFound))
		return true
	}

	return dbError(w, err, db)
}

// QueryCount is used for queries that consist of count in select statement
func QueryCount(db httputil.SqlxDB, query string, args ...interface{}) (*Count, error) {
	var dest Count
	err := db.Get(&dest, query, args...)
	return &dest, err
}
