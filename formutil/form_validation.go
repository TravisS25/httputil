package formutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"

	"github.com/TravisS25/httputil/queryutil"

	"github.com/pkg/errors"

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

var (
	// AllowCacheConfig is global flag that should only be switched to false for testing purposes
	// If this flag is set to false, it will skip getting things from cache for any
	// of the validate functions below
	// The purpose of this is to simulate your cache backend failing and being forced
	// to use database for data
	AllowCacheConfig = true
)

var (
	// ErrBodyMessage is used for when a post/put request does not contain a body in request
	ErrBodyMessage = errors.New("Must have body")

	// ErrInvalidJSON is used when there is an error unmarshalling a struct
	ErrInvalidJSON = errors.New("Invalid json")
)

// Custom error messages used for form validation
const (
	errUnique       = "%s already exists"
	errRequired     = "%s is required"
	errDoesNotExist = "%s does not exist"
)

const (
	RequiredTxt = "Required"
	//MustBeUniqueTxt      = "Must be unique"
	AlreadyExistsTxt     = "Already exists"
	DoesNotExistTxt      = "Does not exist"
	InvalidTxt           = "Invalid"
	InvalidFormatTxt     = "Invalid format"
	InvalidFutureDateTxt = "Date can't be after current date/time"
	InvalidPastDateTxt   = "Date can't be before current date/time"
	CantBeNegativeTxt    = "Can't be negative"
)

//----------------------- INTERFACES ------------------------------

// Validator is main interface that should be used within you testing
// and within your http.HandleFunc routing
type Validator interface {
	Validate(item interface{}) error
}

type ValidatorV2 interface {
	Validate(req *http.Request) (interface{}, error)
}

type RequestValidator interface {
	Validate(req *http.Request, instance interface{}) (interface{}, error)
}

//----------------------- TYPES ------------------------------

type Boolean struct {
	value bool
}

func (c *Boolean) UnmarshalJSON(data []byte) error {
	asString := string(data)
	convertedBool, err := strconv.ParseBool(asString)

	if err != nil {
		c.value = false
	} else {
		c.value = convertedBool
	}

	return nil
}

func (c Boolean) Value() bool {
	return c.value
}

type Int64 int64

func (i Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(i), 10))
}

func (i *Int64) UnmarshalJSON(b []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		value, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*i = Int64(value)
		return nil
	}

	// Fallback to number
	return json.Unmarshal(b, (*int64)(i))
}

func (i Int64) Value() int64 {
	return int64(i)
}

// FormSelection is generic struct used for html forms
type FormSelection struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

type CockroachFormSelection struct {
	Text  string `json:"text"`
	Value int64  `json:"value,string"`
}

//----------------------- VALIDATION RULES ------------------------------

// FormValidation is the main struct that other structs will
// embed to validate json data.  It is also the struct that
// implements SetQuerier and SetCache of Form interface
type FormValidation struct {
	Entity httputil.Entity
	Cache  cacheutil.CacheStore

	db    httputil.Querier
	cache cacheutil.CacheStore
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
	layout,
	timezone string,
	canBeFuture,
	canBePast bool,
) *validateDateRule {
	return &validateDateRule{
		layout:      layout,
		timezone:    timezone,
		canBeFuture: canBeFuture,
		canBePast:   canBePast,
	}
}

// ValidateIDs takes a list of arguments and queries against the querier type given and returns an validateIDsRule instance
// to indicate whether the ids are valid or not
// If the only placeholder parameter within your query is the ids validating against, then the args paramter of ValidateIDs
// can be nil
// Note of caution, the ids we are validating against should be the first placeholder parameters within the query passed
//
// If the ids passed happen to be type formutil#Int64, it will extract the values so it can be used against the query properly
//
// The cacheConfig parameter can be nil if you do not need/have a cache backend
func (f *FormValidation) ValidateIDs(
	querier httputil.Querier,
	cacheConfig *cacheutil.CacheValidateConfig,
	placeHolderPosition,
	bindVar int,
	query string,
	args ...interface{},
) *validateIDsRule {
	return &validateIDsRule{
		querier:             querier,
		cacheConfig:         cacheConfig,
		placeHolderPosition: placeHolderPosition,
		bindVar:             bindVar,
		query:               query,
		args:                args,
		message:             InvalidTxt,
	}
}

func (f *FormValidation) ValidateUniqueness(
	querier httputil.Querier,
	cacheConfig *cacheutil.CacheValidateConfig,
	instanceValue interface{},
	placeHolderPosition,
	bindVar int,
	query string,
	args ...interface{},
) *validateUniquenessRule {
	return &validateUniquenessRule{
		instanceValue: instanceValue,
		querier:       querier,
		cacheConfig:   cacheConfig,
		bindVar:       bindVar,
		query:         query,
		args:          args,
		message:       AlreadyExistsTxt,
	}
}

func (f *FormValidation) ValidateExists(
	querier httputil.Querier,
	cacheConfig *cacheutil.CacheValidateConfig,
	placeHolderPosition,
	bindVar int,
	query string,
	args ...interface{},
) *validateExistsRule {
	return &validateExistsRule{
		querier:             querier,
		cacheConfig:         cacheConfig,
		placeHolderPosition: placeHolderPosition,
		bindVar:             bindVar,
		query:               query,
		args:                args,
		message:             DoesNotExistTxt,
	}
}

// GetQuerier returns httputil.Querier
func (f *FormValidation) GetQuerier() httputil.Querier {
	return f.db
}

// GetCache returns cacheutil.CacheStore
func (f *FormValidation) GetCache() cacheutil.CacheStore {
	return f.cache
}

// SetQuerier sets httputil.Querier
func (f *FormValidation) SetQuerier(querier httputil.Querier) {
	f.db = querier
}

// SetCache sets cacheutil.CacheStore
func (f *FormValidation) SetCache(cache cacheutil.CacheStore) {
	f.cache = cache
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
func (f *FormValidation) Unique(formValue, instanceValue, query string, args ...interface{}) (bool, error) {
	// If instance value is the same as the form value, then
	// we return true as the value has not changed from the
	// instance value
	// This is used when editing an entity so we don't query
	// an instance that hasn't changed and mark it as NOT unique
	if instanceValue == formValue {
		return true, nil
	}

	var filler string
	var err error

	err = f.db.QueryRow(query, args...).Scan(&filler)

	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}

		return false, err
	}

	return false, nil
}

// ValidIDs checks if the query given, which should consist of trying to find
// ids, equals the total number of args passed
// If the number of arguments passed equals the number of rows returned, then
// we return true else returns false
func (f *FormValidation) ValidIDs(query string, args ...interface{}) (bool, error) {
	var rower httputil.Rower
	var err error
	q, arguments, err := sqlx.In(query, args...)

	if err != nil {
		return false, err
	}

	q = sqlx.Rebind(sqlx.DOLLAR, q)
	rower, err = f.db.Query(q, arguments...)

	if err != nil {
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
	var err error

	if f.Entity != nil {
		err = f.Entity.QueryRow(query, args...).Scan(&filler)
	} else {
		err = f.db.QueryRow(query, args...).Scan(&filler)
	}

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
	_, isNil := validation.Indirect(value)
	if validation.IsEmpty(value) || isNil {
		return nil
	}

	var currentTime *time.Time
	var err error
	var message string
	var dateValue string

	switch value.(type) {
	case string:
		dateValue = value.(string)
	case *string:
		temp := value.(*string)

		if temp == nil {
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
		return errors.New(InvalidFormatTxt)
	}

	if v.canBeFuture && v.canBePast {
		message = ""
	} else if v.canBeFuture {
		if dateTime.Before(*currentTime) {
			message = InvalidPastDateTxt
		}
	} else if v.canBePast {
		if dateTime.After(*currentTime) {
			message = InvalidFutureDateTxt
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

type validateExistsRule struct {
	querier             httputil.Querier
	cacheConfig         *cacheutil.CacheValidateConfig
	args                []interface{}
	query               string
	bindVar             int
	placeHolderPosition int
	message             string
}

func (v *validateExistsRule) Validate(value interface{}) error {
	var err error
	var filler string

	_, isNil := validation.Indirect(value)
	if validation.IsEmpty(value) || isNil {
		return nil
	}

	args := make([]interface{}, 0)

	if v.cacheConfig != nil && AllowCacheConfig {
		exists, err := v.cacheConfig.Cache.HasKey(v.cacheConfig.Key)

		if err != nil && err != redis.Nil {
			return validation.NewInternalError(err)
		}

		if !exists {
			return errors.New(v.message)
		}

		return nil
	}

	if len(v.args) != 0 {
		args = append(args, v.args...)
	}

	args = httputil.InsertAt(args, value, v.placeHolderPosition)

	q, arguments, err := queryutil.InQueryRebind(sqlx.DOLLAR, v.query, args...)
	if err != nil {
		return validation.NewInternalError(err)
	}

	err = v.querier.QueryRow(q, arguments...).Scan(&filler)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New(v.message)
		}

		return validation.NewInternalError(err)
	}

	return nil
}

func (v *validateExistsRule) Error(message string) *validateExistsRule {
	return &validateExistsRule{
		querier:     v.querier,
		cacheConfig: v.cacheConfig,
		args:        v.args,
		query:       v.query,
		bindVar:     v.bindVar,
		message:     message,
	}
}

type validateUniquenessRule struct {
	querier             httputil.Querier
	cacheConfig         *cacheutil.CacheValidateConfig
	args                []interface{}
	formValue           interface{}
	instanceValue       interface{}
	query               string
	bindVar             int
	message             string
	placeHolderPosition int
}

func (v *validateUniquenessRule) Validate(value interface{}) error {
	var err error
	var filler string

	_, isNil := validation.Indirect(value)
	if validation.IsEmpty(value) || isNil {
		return nil
	}

	if v.instanceValue == value {
		return nil
	}

	alreadyExists := false
	args := make([]interface{}, 0)

	if v.cacheConfig != nil {
		alreadyExists, err = v.cacheConfig.Cache.HasKey(v.cacheConfig.Key)

		if err != nil && err != redis.Nil {
			return validation.NewInternalError(err)
		}

		if alreadyExists {
			return errors.New(v.message)
		}

		return nil
	}

	if len(v.args) != 0 {
		args = append(args, v.args...)
	}

	args = httputil.InsertAt(args, value, v.placeHolderPosition)

	q, arguments, err := queryutil.InQueryRebind(sqlx.DOLLAR, v.query, args...)
	if err != nil {
		return validation.NewInternalError(err)
	}

	err = v.querier.QueryRow(q, arguments...).Scan(&filler)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return validation.NewInternalError(err)
	}

	return errors.New(v.message)
}

func (v *validateUniquenessRule) Error(message string) *validateUniquenessRule {
	return &validateUniquenessRule{
		querier:             v.querier,
		cacheConfig:         v.cacheConfig,
		args:                v.args,
		formValue:           v.formValue,
		instanceValue:       v.instanceValue,
		query:               v.query,
		bindVar:             v.bindVar,
		message:             message,
		placeHolderPosition: v.placeHolderPosition,
	}
}

type validateIDsRule struct {
	querier             httputil.Querier
	cacheConfig         *cacheutil.CacheValidateConfig
	args                []interface{}
	query               string
	bindVar             int
	message             string
	placeHolderPosition int
	internalError       validation.InternalError
}

func (v *validateIDsRule) Validate(value interface{}) error {
	var err error
	var ids []interface{}
	var expectedLen int
	var singleVal interface{}
	emptySlice := false

	_, isNil := validation.Indirect(value)
	if validation.IsEmpty(value) || isNil {
		return nil
	}

	args := make([]interface{}, 0)

	switch value.(type) {
	case []Int64:
		vals := value.([]Int64)

		if len(vals) != 0 {
			expectedLen = len(vals)
			ids = make([]interface{}, 0, len(vals))

			for _, v := range vals {
				ids = append(ids, v.Value())
			}

			//tempArgs = append(args, ids)
		} else {
			emptySlice = true
		}
	case []int:
		vals := value.([]int)

		if len(vals) != 0 {
			expectedLen = len(vals)
			ids = make([]interface{}, 0, len(vals))

			for _, v := range vals {
				ids = append(ids, v)
			}

			//tempArgs = append(args, ids)
		} else {
			emptySlice = true
		}
	default:
		expectedLen = 1
		singleVal = value
	}

	// If type is slice and is empty, simply return nil as we will get an error
	// when trying to query with empty slice
	if emptySlice {
		return nil
	}

	if len(v.args) != 0 {
		args = append(args, v.args...)
	}

	if v.placeHolderPosition > 0 {
		if len(ids) > 0 {
			args = httputil.InsertAt(args, ids, v.placeHolderPosition-1)
		} else {
			args = httputil.InsertAt(args, singleVal, v.placeHolderPosition-1)
		}
	}

	q, arguments, err := queryutil.InQueryRebind(v.bindVar, v.query, args...)

	if err != nil {
		return validation.NewInternalError(err)
	}

	queryFunc := func() error {
		rower, err := v.querier.Query(q, arguments...)

		// fmt.Printf("query: %s\n", q)
		// fmt.Printf("args: %v\n", arguments)

		if err != nil {
			return validation.NewInternalError(fmt.Errorf(
				"query: %s  err: %s", q, err.Error()),
			)
		}

		counter := 0
		for rower.Next() {
			counter++
		}

		if expectedLen != counter {
			fmt.Printf("counter: %v\n", counter)
			fmt.Printf("len: %v\n", expectedLen)
			return errors.New(v.message)
		}

		return nil
	}

	if v.cacheConfig != nil && AllowCacheConfig {
		var validID bool
		var singleID bool
		var cacheBytes []byte

		if ids == nil {
			singleID = true
			validID, err = v.cacheConfig.Cache.HasKey(v.cacheConfig.Key)
		} else {
			cacheBytes, err = v.cacheConfig.Cache.Get(v.cacheConfig.Key)
		}

		if err != nil && err != redis.Nil {
			err = queryFunc()
		} else {
			if singleID {
				if !validID {
					err = errors.New(v.message)
				}
			} else {
				var cacheIDs []interface{}
				err = json.Unmarshal(cacheBytes, &cacheIDs)

				if err != nil {
					return validation.NewInternalError(err)
				}

				count := 0

				for _, v := range ids {
					for _, t := range cacheIDs {
						if v == t {
							count++
						}
					}
				}

				if count != len(ids) {
					err = errors.New(v.message)
				}
			}
		}
	} else {
		err = queryFunc()
	}

	return err
}

func (v *validateIDsRule) Error(message string) *validateIDsRule {
	return &validateIDsRule{
		message:             message,
		querier:             v.querier,
		cacheConfig:         v.cacheConfig,
		bindVar:             v.bindVar,
		query:               v.query,
		args:                v.args,
		placeHolderPosition: v.placeHolderPosition,
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

func HasFormErrors(w http.ResponseWriter, err error) bool {
	if err != nil {
		//httputil.CheckError(err, "")
		switch err {
		case ErrBodyMessage:
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte(ErrBodyMessage.Error()))
		case ErrInvalidJSON:
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte(ErrInvalidJSON.Error()))
		default:
			if payload, ok := err.(validation.Errors); ok {
				w.WriteHeader(http.StatusNotAcceptable)
				jsonString, _ := json.Marshal(payload)
				w.Write(jsonString)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		return true
	}

	return false
}

func CheckBodyAndDecode(req *http.Request, form interface{}) error {
	if req.Body != nil {
		dec := json.NewDecoder(req.Body)
		err := dec.Decode(&form)

		if err != nil {
			return ErrInvalidJSON
		}
	} else {
		if req.Method == http.MethodDelete {
			return ErrBodyMessage
		}
	}

	return nil
}
