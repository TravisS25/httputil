package formutil

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/cacheutil"
	"github.com/jmoiron/sqlx"
)

const (
	errUnique       = "%s already exists"
	errRequired     = "%s is required"
	errDoesNotExist = "%s does not exist"
)

//----------------------- INTERFACES ------------------------------

type FormValidator interface {
	SetQuerier(querier httputil.Querier)
	SetCache(cache cacheutil.CacheStore)
	Validate() error
}

//----------------------- TYPES ------------------------------

type ConvertibleBoolean struct {
	value bool
}

func (c *ConvertibleBoolean) UnmarshalJSON(data []byte) error {
	asString := string(data)
	convertedBool, err := strconv.ParseBool(asString)

	if err != nil {
		c.value = false
	} else {
		c.value = convertedBool
	}

	return nil
}

func (c ConvertibleBoolean) Value() bool {
	return c.value
}

// type UnmarshalIntPtr struct {
// 	value *int
// }

// func (m UnmarshalIntPtr) UnmarshalJSON(data []byte) error {
// 	fmt.Println("helllllo")
// 	if data == nil {
// 		m.value = nil
// 		return nil
// 	}

// 	asString := string(data)
// 	convertedInt, err := strconv.ParseInt(asString, 10, 32)

// 	if err != nil {
// 		return err
// 	}

// 	value := int(convertedInt)
// 	m.value = &value
// 	return nil
// }

// func (m UnmarshalIntPtr) Value() *int {
// 	return m.value
// }

type FormSelection struct {
	Text  string      `json:"text"`
	Value interface{} `json:"value"`
}

type FormValidation struct {
	db    httputil.Querier
	cache cacheutil.CacheStore
}

func (m *FormValidation) GetQuerier() httputil.Querier {
	return m.db
}

func (m *FormValidation) GetCache() cacheutil.CacheStore {
	return m.cache
}

func (m *FormValidation) SetQuerier(querier httputil.Querier) {
	m.db = querier
}

func (m *FormValidation) SetCache(cache cacheutil.CacheStore) {
	m.cache = cache
}

func (m *FormValidation) IsValid(isValid bool) *validRule {
	return &validRule{isValid: isValid, message: "Not Valid"}
}

func (m *FormValidation) RequiredError(field string) string {
	return fmt.Sprintf(errRequired, field)
}

func (m *FormValidation) UniqueError(field string) string {
	return fmt.Sprintf(errUnique, field)
}

func (m *FormValidation) ExistError(field string) string {
	return fmt.Sprintf(errDoesNotExist, field)
}

func (m *FormValidation) Unique(formValue string, instanceValue string, query string, args ...interface{}) bool {
	if instanceValue == formValue {
		return true
	}

	var filler string
	err := m.db.QueryRow(query, args...).Scan(&filler)

	if err != sql.ErrNoRows {
		return false
	}

	return true
}

func (f *FormValidation) ValidIDs(query string, args ...interface{}) (bool, error) {
	q, arguments, err := sqlx.In(query, args...)

	if err != nil {
		return false, err
	}

	q = sqlx.Rebind(sqlx.DOLLAR, q)
	rower, err := f.db.Query(q, arguments...)

	if err != nil {
		fmt.Printf("err: %s", err)
		return false, err
	}

	counter := 0
	for rower.Next() {
		counter++
	}

	if len(arguments) != counter {
		fmt.Printf("len arg: %d; len counter: %d\n", len(arguments), counter)
		fmt.Printf("slice: %d", arguments)
		return false, nil
	}

	return true, nil
}

func (m *FormValidation) Exists(query string, args ...interface{}) bool {
	var filler string
	err := m.db.QueryRow(query, args...).Scan(&filler)

	if err == sql.ErrNoRows {
		return false
	}

	return true
}

type validRule struct {
	isValid bool
	message string
}

func (v *validRule) Validate(value interface{}) error {
	if !v.isValid {
		return errors.New(v.message)
	}

	return nil
}

// Error sets the error message for the rule.
func (v *validRule) Error(message string) *validRule {
	return &validRule{
		message: message,
		isValid: v.isValid,
	}
}
