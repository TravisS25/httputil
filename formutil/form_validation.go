package formutil

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/cacheutil"
	"github.com/TravisS25/httputil/timeutil"
	"github.com/go-ozzo/ozzo-validation"
	"github.com/jmoiron/sqlx"
)

var (
	PhoneNumberRegex *regexp.Regexp
	EmailRegex       *regexp.Regexp
	ZipRegex         *regexp.Regexp
	DateRegex        *regexp.Regexp
	ColorRegex       *regexp.Regexp
)

// Custom error messages used for form validation
const (
	errUnique       = "%s already exists"
	errRequired     = "%s is required"
	errDoesNotExist = "%s does not exist"
)

const (
	InvalidDateFormat = "Invalid date format"
	InvalidFutureDate = "Date can't be after current date/time"
	InvalidPastDate   = "Date can't be before current date/time"
)

//----------------------- INTERFACES ------------------------------

type FormValidator interface {
	SetQuerier(querier httputil.Querier)
	SetCache(cache cacheutil.CacheStore)
	Validate() error
}

// Validator is main interface that should be used within you testing
// and within your http.HandleFunc routing
type Validator interface {
	Validate(item interface{}) error
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

// FormSelection is generic struct used for html forms
type FormSelection struct {
	Text  string      `json:"text"`
	Value interface{} `json:"value"`
}

// FormValidation is the main struct that other structs will
// embed to validate json data.  It is also the struct that
// implements SetQuerier and SetCache of Form interface
type FormValidation struct {
	db    httputil.Querier
	cache cacheutil.CacheStore

	// V2 for backwards compatability
	cacheV2 cacheutil.CacheStoreV2
}

// GetQuerier returns httputil.Querier
func (f *FormValidation) GetQuerier() httputil.Querier {
	return f.db
}

// GetCache returns cacheutil.CacheStore
func (f *FormValidation) GetCache() cacheutil.CacheStore {
	return f.cache
}

// GetCacheV2 returns cacheutil.CacheStoreV2
func (f *FormValidation) GetCacheV2() cacheutil.CacheStoreV2 {
	return f.cacheV2
}

// SetQuerier sets httputil.Querier
func (f *FormValidation) SetQuerier(querier httputil.Querier) {
	f.db = querier
}

// SetCache sets cacheutil.CacheStore
func (f *FormValidation) SetCache(cache cacheutil.CacheStore) {
	f.cache = cache
}

// SetCacheV2 sets cacheutil.CacheStoreV2
func (f *FormValidation) SetCacheV2(cache cacheutil.CacheStoreV2) {
	f.cacheV2 = cache
}

// IsValid returns *validRule based on isValid parameter
// Basically IsValid is a wrapper for the passed bool
// to return valid rule to then apply custom error message
// for the Error function
func (f *FormValidation) IsValid(isValid bool) *validRule {
	return &validRule{isValid: isValid, message: "Not Valid"}
}

// ValidateDate verifies whether a date string matches the passed in
// layout format
// If a user wishes, they can also verify whether the given date is
// allowed to be a past or future date of the current time
// The timezone parameter converts given time to compare to current
// time if you choose to
// If no timezone is passed, UTC is used by default
// If user does not want to compare time, both bool parameters
// should be true
// Will raise validation.InternalError if both bool parameters are false
func (f *FormValidation) ValidateDate(
	// dateType int,
	layout,
	timezone string,
	canBeFuture,
	canBePast bool,
) *validateDateRule {
	return &validateDateRule{
		// dateType:    dateType,
		layout:      layout,
		timezone:    timezone,
		canBeFuture: canBeFuture,
		canBePast:   canBePast,
	}
}

// RequiredError is wrapper for the field parameter
// Returns field name with custom required message
func (f *FormValidation) RequiredError(field string) string {
	return fmt.Sprintf(errRequired, field)
}

// UniqueError is wrapper for field parameter
// Returns field name with custom message stating that
// the field is not unique
func (f *FormValidation) UniqueError(field string) string {
	return fmt.Sprintf(errUnique, field)
}

// ExistError is wrapper for field parameter
// Returns field name with custom message stating that
// that the field does not exist
func (f *FormValidation) ExistError(field string) string {
	return fmt.Sprintf(errDoesNotExist, field)
}

// Unique returns true if the given formValue and instanceValue are not
// found in the query given
func (f *FormValidation) Unique(formValue string, instanceValue string, query string, args ...interface{}) bool {
	if instanceValue == formValue {
		return true
	}

	var filler string
	err := f.db.QueryRow(query, args...).Scan(&filler)

	if err != sql.ErrNoRows {
		return false
	}

	return true
}

// ValidIDs checks if the query given, which should consist of trying to find
// ids, equals the total number of args passed
// If the number of arguments passed equals the number of rows returned, then
// we return true else returns false
func (f *FormValidation) ValidIDs(query string, args ...interface{}) (bool, error) {
	q, arguments, err := sqlx.In(query, args...)

	if err != nil {
		return false, err
	}

	q = sqlx.Rebind(sqlx.DOLLAR, q)
	rower, err := f.db.Query(q, arguments...)

	if err != nil {
		//fmt.Printf("err: %s", err)
		return false, err
	}

	counter := 0
	for rower.Next() {
		counter++
	}

	if len(arguments) != counter {
		return false, nil
	}

	return true, nil
}

// Exists returns true if given query returns a row from database
// Else return false
func (f *FormValidation) Exists(query string, args ...interface{}) (bool, error) {
	var filler string
	err := f.db.QueryRow(query, args...).Scan(&filler)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

type validateDateRule struct {
	layout        string
	timezone      string
	canBeFuture   bool
	canBePast     bool
	internalError validation.InternalError
}

func (v *validateDateRule) Validate(value interface{}) error {
	if v.internalError != nil {
		return v.internalError
	}

	var currentTime *time.Time
	var err error
	var message string
	var dateValue string

	switch value.(type) {
	case string:
		dateValue = value.(string)
		fmt.Println("made to string")
	case *string:
		fmt.Println("made to *string")
		temp := value.(*string)

		if temp == nil {
			fmt.Println("niiiiiil")
			return nil
		}

		dateValue = *temp
	default:
		return validation.NewInternalError(errors.New("Input must be string or *string"))
	}

	if v.timezone != "" {
		currentTime, err = timeutil.GetCurrentLocalDateTimeInUTC(v.timezone)

		if err != nil {
			return validation.NewInternalError(err)
		}
	} else {
		current := time.Now().UTC()
		currentTime = &current
	}

	dateTime, err := time.Parse(v.layout, dateValue)

	if err != nil {
		return errors.New(InvalidDateFormat)
	}

	if v.canBeFuture && v.canBePast {
		message = ""
	} else if v.canBeFuture {
		if dateTime.Before(*currentTime) {
			message = "Date can't be before current date/time"
		}
	} else if v.canBePast {
		if dateTime.After(*currentTime) {
			message = "Date can't be after current date/time"
		}
	} else {
		return validation.NewInternalError(
			errors.New("Both 'canBeFuture and 'canBePast' can't be false"),
		)
	}

	if message != "" {
		return errors.New(message)
	}

	return nil
}

func (v *validateDateRule) Error(message string) *validateDateRule {
	return &validateDateRule{
		layout:        v.layout,
		timezone:      v.timezone,
		canBeFuture:   v.canBeFuture,
		canBePast:     v.canBePast,
		internalError: v.internalError,
	}
}

type validRule struct {
	isValid       bool
	internalError validation.InternalError
	message       string
}

func (v *validRule) Validate(value interface{}) error {
	if v.internalError != nil {
		return v.internalError
	}
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

//----------------------- FUNCTIONS ------------------------------

func StandardizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func init() {
	initRegexExpressions()
}

func initRegexExpressions() {
	EmailRegex, _ = regexp.Compile("^.+@[a-zA-Z0-9.]+$")
	ZipRegex, _ = regexp.Compile("^[0-9]{5}$")
	PhoneNumberRegex, _ = regexp.Compile("^\\([0-9]{3}\\)-[0-9]{3}-[0-9]{4}$")
	ColorRegex, _ = regexp.Compile("^#[0-9a-z]{6}$")
}
