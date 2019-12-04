package queryutil

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/TravisS25/httputil/cacheutil"
	"github.com/TravisS25/httputil/confutil"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/dbutil"
	"github.com/knq/snaker"

	"github.com/jmoiron/sqlx"

	"reflect"

	"github.com/pkg/errors"
)

const (
	// Select string for queries
	Select = "select "
)

// Aggregate Types
const (
	AggregateCount = iota + 1
	AggregateSum
	AggregateAverage
	AggregateMin
	AggregateMax
)

var (
	ErrInvalidSort  = errors.New("invalid sort")
	ErrInvalidArray = errors.New("invalid array for field")
	ErrInvalidValue = errors.New("invalid field value")
)

////////////////////////////////////////////////////////////
// MISC
////////////////////////////////////////////////////////////

// FormRequest is used to get form values from url string
// Will mostly come from http.Request
type FormRequest interface {
	FormValue(string) string
}

////////////////////////////////////////////////////////////
// CUSTOM ERRORS
////////////////////////////////////////////////////////////

type FilterError struct {
	invalidOperation bool
	invalidFilter    bool
	invalidValue     bool

	field string
	value interface{}
}

func (f *FilterError) Error() string {
	if f.invalidFilter {
		return fmt.Sprintf("invalid filter field: '%s'", f.field)
	}
	if f.invalidValue {
		return fmt.Sprintf("invalid value '%v' for filter '%s'", f.value, f.field)
	}
	if f.invalidOperation {
		return fmt.Sprintf("invalid filter operation for field: '%s'", f.field)
	}

	return ""
}

func (f *FilterError) isFilterError() bool {
	return f.invalidFilter
}

func (f *FilterError) isValueError() bool {
	return f.invalidValue
}

func (f *FilterError) isOperationError() bool {
	return f.invalidOperation
}

func (f *FilterError) setInvalidFilterError(field string) {
	f.field = field
	f.invalidFilter = true
}

func (f *FilterError) setInvalidValueError(field string, value interface{}) {
	f.field = field
	f.value = value
	f.invalidValue = true
}

func (f *FilterError) setInvalidOperationError(field string) {
	f.field = field
	f.invalidOperation = true
}

type SortError struct {
	invalidOperation bool
	invalidSort      bool
	invalidDir       bool

	field string
	value string
}

func (s *SortError) Error() string {
	if s.invalidSort {
		return fmt.Sprintf("invalid sort field: '%s'", s.field)
	}
	if s.invalidDir {
		return fmt.Sprintf("invalid sort dir '%s' for field '%s'", s.value, s.field)
	}

	return ""
}

func (s *SortError) isSortError() bool {
	return s.invalidSort
}

func (s *SortError) isDirError() bool {
	return s.invalidDir
}

func (s *SortError) isOperationError() bool {
	return s.invalidOperation
}

func (s *SortError) setInvalidSortError(field string) {
	s.field = field
	s.invalidSort = true
}

func (s *SortError) setInvalidDirError(field, dir string) {
	s.field = field
	s.value = dir
	s.invalidDir = true
}

func (f *SortError) setInvalidOperationError(field string) {
	f.field = field
	f.invalidOperation = true
}

type GroupError struct {
	invalidField bool

	field string
}

func (s *GroupError) Error() string {
	if s.invalidField {
		return fmt.Sprintf("invalid group field: '%s'", s.field)
	}

	return ""
}

func (s *GroupError) isGroupError() bool {
	return s.invalidField
}

func (s *GroupError) setInvalidGroupError(field string) {
	s.field = field
	s.invalidField = true
}

type SliceError struct {
	invalidSlice bool

	fieldType string
	field     string
}

func (s *SliceError) Error() string {
	if s.invalidSlice {
		return fmt.Sprintf("invalid type (%s) within array for field: '%s'", s.fieldType, s.field)
	}

	return ""
}

func (s *SliceError) isSliceError() bool {
	return s.invalidSlice
}

func (s *SliceError) setInvalidSliceError(field, fieldType string) {
	s.field = field
	s.fieldType = fieldType
	s.invalidSlice = true
}

////////////////////////////////////////////////////////////
// CONFIG STRUCTS
////////////////////////////////////////////////////////////

type filterResult struct {
	Replacements []interface{}
	Filters      []Filter
}

type sortResult struct {
	Replacements []interface{}
	Sorts        []Sort
}

type groupResult struct {
	Replacements []interface{}
	Groups       []Group
}

type resultReplacements struct {
	Filters      []Filter
	Sorts        []Sort
	Groups       []Group
	Replacements []interface{}
}

// OperationConfig is used in conjunction with FieldConfig{}
// to determine if the field associated can perform certain
// sql actions
type OperationConfig struct {
	// CanFilterBy determines whether field can have filters applied
	CanFilterBy bool

	// CanSortBy determines whether field can be sorted
	CanSortBy bool

	// CanGroupBy determines whether field can be grouped
	CanGroupBy bool
}

// FieldConfig is meant to be a per database field config
// to determine if a user can perform a certain sql action
// and if user tries to perform action not allowed, throw error
type FieldConfig struct {
	// DBField should be the name of the database field
	// to apply configurations to
	DBField string

	// OperationConf is config to set to determine which sql
	// operations can be performed on DBField
	OperationConf OperationConfig
}

// ParamConfig is for extracting expected query params from url
// to be passed to the server
type ParamConfig struct {
	// Filter is for query param that will be applied
	// to "where" clause of query
	Filter *string

	// Sort is for query param that will be applied
	// to "order by" clause of query
	Sort *string

	// Take is for query param that will be applied
	// to "limit" clause of query
	Take *string

	// Skip is for query param that will be applied
	// to "offset" clause of query
	Skip *string

	// Group is for query param that will be applied
	// to "group by" clause of query
	Group *string
}

// QueryConfig is config for how the overall execution of the query
// is supposed to be performed
type QueryConfig struct {
	// SQLBindVar is used to determines what query placeholder parameters
	// will be converted to depending on what database being used
	// This is based off of the sqlx library
	SQLBindVar *int

	// TakeLimit is used to set max limit on number of
	// records that are returned from query
	TakeLimit *int

	// PrependFilterFields prepends filters to query before
	// ones passed by url query params
	PrependFilterFields []Filter

	// PrependGroupFields prepends groups to query before
	// ones passed by url query params
	PrependGroupFields []Group

	// PrependSortFields prepends sorts to query before
	// ones passed by url query params
	PrependSortFields []Sort

	// ExcludeFilters determines whether to exclude applying
	// filters from url query params
	// The PrependFilterFields property is NOT effected by this
	ExcludeFilters bool

	// ExcludeGroups determines whether to exclude applying
	// groups from url query params
	// The PrependGroupFields property is NOT effected by this
	ExcludeGroups bool

	// ExcludeSorts determines whether to exclude applying
	// sorts from url query params
	// The PrependSortFields property is NOT effected by this
	ExcludeSorts bool

	// ExcludeLimitWithOffset determines whether to exclude applying
	// limit and offset from url query params
	ExcludeLimitWithOffset bool

	// DisableGroupMod is used to determine if a user wants to disable
	// a query from automatically being modified to accommodate a
	// group by with order by without the client having to explictly send
	// group by parameters along with order by
	//
	// In sql, if you have a group by and order by, the order by field(s)
	// also have to appear in group by
	// The GetPreQueryResults() function and functions that utilize it will
	// automatically add the order by fields to the group by clause if they are
	// needed unless DisableGroupMod is set true
	DisableGroupMod bool
}

type ApplyConfig struct {
	ApplyLimit        bool
	ApplyOrdering     bool
	ExecuteQuery      bool
	ExecuteCountQuery bool
	ExclusionFields   []string
}

////////////////////////////////////////////////////////////
// QUERY STRUCTS
////////////////////////////////////////////////////////////

// Filter is the filter config struct for server side filtering
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// Sort is the sort config struct for server side sorting
type Sort struct {
	Dir   string `json:"dir"`
	Field string `json:"field"`
}

// Aggregate is config struct to be used in conjunction with Group
// type Aggregate struct {
// 	Field     string
// 	Aggregate int
// }

// Group is the group config struct for server side grouping
type Group struct {
	// Dir        string       `json:"dir"`
	Field string `json:"field"`
	// Aggregates []*Aggregate `json:"aggregates"`
}

////////////////////////////////////////////////////////
// NEW
////////////////////////////////////////////////////////

func getResults(
	query *string,
	db httputil.Querier,
	queryConf QueryConfig,
	prependVars []interface{},
	filterReplacements []interface{},
	limitOffsetReplacements []interface{},
) ([]interface{}, error) {
	var replacements []interface{}
	var err error

	totalReplacements := len(filterReplacements)

	if prependVars != nil {
		totalReplacements += len(prependVars)
	}

	if limitOffsetReplacements != nil {
		totalReplacements += len(limitOffsetReplacements)
	}

	replacements = make([]interface{}, 0, totalReplacements)

	if prependVars != nil {
		//fmt.Printf("prependVars: %v\n", prependVars)
		for _, v := range prependVars {
			replacements = append(replacements, v)
		}
	}

	for _, v := range filterReplacements {
		replacements = append(replacements, v)
	}

	if limitOffsetReplacements != nil {
		for _, v := range limitOffsetReplacements {
			replacements = append(replacements, v)
		}
	}

	// fmt.Printf("query at this point: %s\n", *query)
	// fmt.Printf("replacements overall: %v\n", replacements)

	if *query, replacements, err = InQueryRebind(
		*queryConf.SQLBindVar, *query, replacements...,
	); err != nil {
		return nil, errors.Wrap(err, "\n-------------------\n")
	}

	return replacements, nil
}

func getCountResults(
	query *string,
	db httputil.Querier,
	queryConf QueryConfig,
	prependVars []interface{},
	filterReplacements []interface{},
	limitOffsetReplacements []interface{},
) (int, error) {
	replacements, err := getResults(
		query,
		db,
		queryConf,
		prependVars,
		filterReplacements,
		limitOffsetReplacements,
	)

	if err != nil {
		return 0, err
	}

	rower, err := db.Query(*query, replacements...)

	if err != nil {
		return 0, err
	}

	totalCount := 0

	for rower.Next() {
		var count int
		err = rower.Scan(&count)

		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil
			}

			return 0, err
		}

		totalCount += count
	}

	return totalCount, nil
}

func getReplacementResults(
	query *string,
	countQuery *string,
	r FormRequest,
	paramConf *ParamConfig,
	queryConf *QueryConfig,
	fields map[string]FieldConfig,
) (*resultReplacements, error) {
	var q *string
	var filterReplacements []interface{}
	var filters []Filter
	var sorts []Sort
	var groups []Group
	var err error

	f := "filters"
	sk := "skip"
	so := "sorts"
	t := "take"
	g := "groups"

	sql := sqlx.QUESTION
	limit := 100

	if paramConf.Filter == nil {
		paramConf.Filter = &f
	}
	if paramConf.Skip == nil {
		paramConf.Skip = &sk
	}
	if paramConf.Sort == nil {
		paramConf.Sort = &so
	}
	if paramConf.Take == nil {
		paramConf.Take = &t
	}
	if paramConf.Group == nil {
		paramConf.Group = &g
	}

	if queryConf.SQLBindVar == nil {
		queryConf.SQLBindVar = &sql
	}
	if queryConf.TakeLimit == nil {
		queryConf.TakeLimit = &limit
	}

	if query != nil {
		q = query
	} else {
		q = countQuery
	}

	if filters, filterReplacements, err = GetFilterReplacements(
		r,
		q,
		*paramConf.Filter,
		*queryConf,
		// queryConf.ExcludeFilters,
		// queryConf.PrependFilterFields,
		fields,
	); err != nil {
		return nil, errors.Wrap(err, "")
	}

	if groups, err = GetGroupReplacements(
		r,
		q,
		*paramConf.Group,
		*queryConf,
		// queryConf.PrependGroupFields,
		fields,
	); err != nil {
		return nil, errors.Wrap(err, "")
	}

	if query != nil {
		if sorts, err = DecodeSorts(r, *paramConf.Sort); err != nil {
			return nil, errors.Wrap(err, "")
		}

		if len(groups) > 0 && len(sorts) > 0 && !queryConf.DisableGroupMod {
			groupFields := make([]string, 0)

			for _, v := range sorts {
				hasGroupInSort := false
				conf, ok := fields[v.Field]

				if ok {
					if conf.OperationConf.CanSortBy {
						for _, k := range groups {
							if v.Field == k.Field {
								hasGroupInSort = true
							}
						}

						if !hasGroupInSort && len(sorts) > 0 {
							groupFields = append(groupFields, fields[v.Field].DBField)
						}
					} else {
						sortErr := &SortError{}
						sortErr.setInvalidOperationError(v.Field)
						return nil, sortErr
					}
				} else {
					sortErr := &SortError{}
					sortErr.setInvalidSortError(v.Field)
					return nil, sortErr
				}
			}

			for i, v := range groupFields {
				if i == 0 {
					*q += ","
				}

				*q += " " + v

				if i != len(groupFields)-1 {
					*q += ","
				}
			}

		}

		if sorts, err = GetSortReplacements(
			r,
			q,
			*paramConf.Sort,
			*queryConf,
			fields,
		); err != nil {
			return nil, errors.Wrap(err, "")
		}
	}

	return &resultReplacements{
		Filters:      filters,
		Groups:       groups,
		Sorts:        sorts,
		Replacements: filterReplacements,
	}, nil
}

// func getReplacementResults(
// 	query *string,
// 	countQuery *string,
// 	r FormRequest,
// 	paramConf *ParamConfig,
// 	queryConf *QueryConfig,
// 	fields map[string]FieldConfig,
// ) (*resultReplacements, error) {
// 	var q *string
// 	var filterReplacements []interface{}
// 	var filters []Filter
// 	var sorts []Sort
// 	var groups []Group
// 	var err error

// 	f := "filters"
// 	sk := "skip"
// 	so := "sorts"
// 	t := "take"
// 	g := "groups"

// 	sql := sqlx.QUESTION
// 	limit := 100

// 	if paramConf.Filter == nil {
// 		paramConf.Filter = &f
// 	}
// 	if paramConf.Skip == nil {
// 		paramConf.Skip = &sk
// 	}
// 	if paramConf.Sort == nil {
// 		paramConf.Sort = &so
// 	}
// 	if paramConf.Take == nil {
// 		paramConf.Take = &t
// 	}
// 	if paramConf.Group == nil {
// 		paramConf.Group = &g
// 	}

// 	if queryConf.SQLBindVar == nil {
// 		queryConf.SQLBindVar = &sql
// 	}
// 	if queryConf.TakeLimit == nil {
// 		queryConf.TakeLimit = &limit
// 	}

// 	if query != nil{
// 		q = query
// 	} else{
// 		q = countQuery
// 	}

// 	if filters, filterReplacements, err = GetFilterReplacements(
// 		r,
// 		q,
// 		*paramConf.Filter,
// 		*queryConf,
// 		// queryConf.ExcludeFilters,
// 		// queryConf.PrependFilterFields,
// 		fields,
// 	); err != nil {
// 		return nil, errors.Wrap(err, "")
// 	}

// 	if groups, err = GetGroupReplacements(
// 		r,
// 		q,
// 		*paramConf.Group,
// 		*queryConf,
// 		// queryConf.PrependGroupFields,
// 		fields,
// 	); err != nil {
// 		return nil, errors.Wrap(err, "")
// 	}

// 	if query != nil {
// 		if sorts, err = DecodeSorts(r, *paramConf.Sort); err != nil {
// 			return nil, errors.Wrap(err, "")
// 		}

// 		if len(groups) > 0 && len(sorts) > 0 && !queryConf.DisableGroupMod {
// 			groupFields := make([]string, 0)

// 			for _, v := range sorts {
// 				hasGroupInSort := false
// 				conf, ok := fields[v.Field]

// 				if ok {
// 					if conf.OperationConf.CanSortBy {
// 						for _, k := range groups {
// 							if v.Field == k.Field {
// 								hasGroupInSort = true
// 							}
// 						}

// 						if !hasGroupInSort && len(sorts) > 0 {
// 							groupFields = append(groupFields, fields[v.Field].DBField)
// 						}
// 					} else {
// 						sortErr := &SortError{}
// 						sortErr.setInvalidOperationError(v.Field)
// 						return nil, sortErr
// 					}
// 				} else {
// 					sortErr := &SortError{}
// 					sortErr.setInvalidSortError(v.Field)
// 					return nil, sortErr
// 				}
// 			}

// 			for i, v := range groupFields {
// 				if i == 0 {
// 					*query += ","
// 				}

// 				*query += " " + v

// 				if i != len(groupFields)-1 {
// 					*query += ","
// 				}
// 			}

// 		}

// 		if sorts, err = GetSortReplacements(
// 			r,
// 			q,
// 			*paramConf.Sort,
// 			*queryConf,
// 			fields,
// 		); err != nil {
// 			return nil, errors.Wrap(err, "")
// 		}
// 	}

// 	return &resultReplacements{
// 		Filters:      filters,
// 		Groups:       groups,
// 		Sorts:        sorts,
// 		Replacements: filterReplacements,
// 	}, nil
// }

func GetQueriedAndCountResults(
	query *string,
	countQuery *string,
	prependVars []interface{},
	fields map[string]FieldConfig,
	r FormRequest,
	db httputil.Querier,
	paramConf ParamConfig,
	queryConf QueryConfig,
) (httputil.Rower, int, error) {
	rower, err := GetQueriedResults(
		query,
		prependVars,
		fields,
		r,
		db,
		paramConf,
		queryConf,
	)

	if err != nil {
		return nil, 0, errors.Wrap(err, "")
	}

	//fmt.Printf("query: %s\n", *query)

	count, err := GetCountResults(
		countQuery,
		prependVars,
		fields,
		r,
		db,
		paramConf,
		queryConf,
	)

	//fmt.Printf("count query: %s\n", *countQuery)

	if err != nil {
		return nil, 0, errors.Wrap(err, "")
	}

	return rower, count, nil
}

func GetCountResults(
	countQuery *string,
	prependVars []interface{},
	fields map[string]FieldConfig,
	r FormRequest,
	db httputil.Querier,
	paramConf ParamConfig,
	queryConf QueryConfig,
) (int, error) {
	//var replacements []interface{}
	var results *resultReplacements
	var err error

	if results, err = getReplacementResults(
		nil,
		countQuery,
		r,
		&paramConf,
		&queryConf,
		fields,
	); err != nil {
		return 0, errors.Wrap(err, "")
	}

	return getCountResults(
		countQuery,
		db,
		queryConf,
		prependVars,
		results.Replacements,
		nil,
	)
}

func GetPreQueryResults(
	query *string,
	prependVars []interface{},
	fields map[string]FieldConfig,
	r FormRequest,
	db httputil.Querier,
	paramConf ParamConfig,
	queryConf QueryConfig,
) ([]interface{}, error) {
	var results *resultReplacements
	//var replacements []interface{}
	var limitOffsetReplacements []interface{}
	var err error

	if results, err = getReplacementResults(
		query,
		nil,
		r,
		&paramConf,
		&queryConf,
		fields,
	); err != nil {
		return nil, errors.Wrap(err, "")
	}

	if !queryConf.ExcludeLimitWithOffset {
		if limitOffsetReplacements, err = GetLimitWithOffsetReplacements(
			r,
			query,
			*paramConf.Take,
			*paramConf.Skip,
			*queryConf.TakeLimit,
		); err != nil {
			return nil, errors.Wrap(err, "")
		}
	}

	replacements, err := getResults(
		query,
		db,
		queryConf,
		prependVars,
		results.Replacements,
		limitOffsetReplacements,
	)

	if err != nil {
		return nil, err
	}

	return replacements, nil
}

func GetQueriedResults(
	query *string,
	prependVars []interface{},
	fields map[string]FieldConfig,
	r FormRequest,
	db httputil.Querier,
	paramConf ParamConfig,
	queryConf QueryConfig,
) (httputil.Rower, error) {
	replacements, err := GetPreQueryResults(
		query,
		prependVars,
		fields,
		r,
		db,
		paramConf,
		queryConf,
	)

	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	return db.Query(*query, replacements...)
}

////////////////////////////////////////////////////////////
// GET REPLACEMENT FUNCTIONS
////////////////////////////////////////////////////////////

// GetFilterReplacements will decode passed paramName paramter and decode it from FormRequest into []Filter
// It will then apply these filters to passed query and return extracted values
// Applies "where" or "and" to query string depending on whether the query string already contains a where clause
// Throws FilterError{} error type if error occurs
func GetFilterReplacements(
	r FormRequest,
	query *string,
	paramName string,
	queryConf QueryConfig,
	// excludeFilters bool,
	// prependFilters []Filter,
	fields map[string]FieldConfig,
) ([]Filter, []interface{}, error) {
	var err error
	var allFilters, filters []Filter
	var replacements, prependReplacements, allReplacements []interface{}

	filterExp := regexp.MustCompile(`(?i)(\n|\t|\s)where(\n|\t|\s)`)

	if queryConf.PrependFilterFields != nil {
		if len(queryConf.PrependFilterFields) > 0 {
			if f := filterExp.FindString(*query); f == "" {
				*query += " where"
			} else {
				*query += " and"
			}

			if prependReplacements, err = ReplaceFilterFields(
				query,
				queryConf.PrependFilterFields,
				fields,
			); err != nil {
				return nil, nil, errors.Wrap(err, "")
			}
		}
	} else {
		queryConf.PrependFilterFields = make([]Filter, 0)
	}

	if !queryConf.ExcludeFilters {
		if filters, err = DecodeFilters(r, paramName); err != nil {
			return nil, nil, errors.Wrap(err, "")
		}

		if len(filters) > 0 {
			if f := filterExp.FindString(*query); f == "" {
				*query += " where"
			} else {
				*query += " and"
			}

			if replacements, err = ReplaceFilterFields(query, filters, fields); err != nil {
				return nil, nil, errors.Wrap(err, "")
			}
		}
	} else {
		filters = make([]Filter, 0)
	}

	allFilters = make([]Filter, 0, len(queryConf.PrependFilterFields)+len(filters))
	allReplacements = make([]interface{}, 0, len(prependReplacements)+len(replacements))

	for _, v := range queryConf.PrependFilterFields {
		allFilters = append(allFilters, v)
	}
	for _, v := range filters {
		allFilters = append(allFilters, v)
	}

	for _, v := range prependReplacements {
		allReplacements = append(allReplacements, v)
	}
	for _, v := range replacements {
		allReplacements = append(allReplacements, v)
	}

	return allFilters, allReplacements, nil
}

// // GetFilterReplacements will decode passed paramName paramter and decode it from FormRequest into []*Filter
// // It will then apply these filters to passed query and return extracted values
// // Applies "where" or "and" to query string depending on whether the query string already contains a where clause
// // Throws FilterError{} error type if error occurs
// func GetFilterReplacements(r FormRequest, query *string, paramName string, fields map[string]FieldConfig) ([]*Filter, []interface{}, error) {
// 	var err error
// 	var filters []*Filter
// 	var replacements []interface{}

// 	if filters, err = DecodeFilters(r, paramName); err != nil {
// 		return nil, nil, err
// 	}

// 	if len(filters) > 0 {
// 		var selectCount, whereCount int

// 		// Regular expression for checking whether the given query
// 		// already has a where statement
// 		selectExp := regexp.MustCompile(`(?i)(\n|\t|\s|\A)select(\n|\t|\s)`)
// 		whereExp := regexp.MustCompile(`(?i)(\n|\t|\s)where(\n|\t|\s)`)

// 		selectSlice := selectExp.FindAllStringIndex(*query, -1)
// 		whereSlice := whereExp.FindAllStringIndex(*query, -1)

// 		if selectSlice != nil {
// 			selectCount = len(selectSlice)
// 		}
// 		if whereSlice != nil {
// 			whereCount = len(whereSlice)
// 		}

// 		if whereCount < selectCount {
// 			*query += " where "
// 		} else {
// 			*query += " and "
// 		}

// 		if replacements, err = ReplaceFilterFields(query, filters, fields); err != nil {
// 			return nil, nil, err
// 		}
// 	} else {
// 		replacements = make([]interface{}, 0)
// 	}

// 	return nil, replacements, nil
// }

// GetSortReplacements will decode passed paramName paramter and decode it from FormRequest into []*Sort
// It will then apply these sorts to passed query and return extracted values
// Will apply "order by" text to query if not found
// Throws SortError{} error type if error occurs
func GetSortReplacements(
	r FormRequest,
	query *string,
	paramName string,
	queryConf QueryConfig,
	// excludeSorts bool,
	// prependSorts []Sort,
	fields map[string]FieldConfig,
) ([]Sort, error) {
	var allSorts, sortSlice []Sort
	//var replacements, prependReplacements []interface{}
	var err error

	orderExp := regexp.MustCompile(`(?i)(\n|\t|\s)order(\n|\t|\s)`)

	if queryConf.PrependSortFields != nil {
		if len(queryConf.PrependSortFields) > 0 {
			if s := orderExp.FindString(*query); s == "" {
				*query += " order by "
			} else {
				*query += ","
			}

			if err = ReplaceSortFields(
				query,
				queryConf.PrependSortFields,
				fields,
			); err != nil {
				return nil, errors.Wrap(err, "")
			}
		}
	} else {
		queryConf.PrependSortFields = make([]Sort, 0)
	}

	if !queryConf.ExcludeSorts {
		if sortSlice, err = DecodeSorts(r, paramName); err != nil {
			return nil, errors.Wrap(err, "")
		}

		if len(sortSlice) > 0 {
			if s := orderExp.FindString(*query); s == "" {
				*query += " order by "
			} else {
				*query += ","
			}

			if err = ReplaceSortFields(query, sortSlice, fields); err != nil {
				return nil, errors.Wrap(err, "")
			}
		}
	}

	allSorts = make([]Sort, 0, len(queryConf.PrependSortFields)+len(sortSlice))
	//allReplacements = make([]interface{}, 0, len(prependReplacements)+len(replacements))

	for _, v := range queryConf.PrependSortFields {
		allSorts = append(allSorts, v)
	}
	for _, v := range sortSlice {
		allSorts = append(allSorts, v)
	}

	// for _, v := range prependReplacements {
	// 	allReplacements = append(allReplacements, v)
	// }
	// for _, v := range replacements {
	// 	allReplacements = append(allReplacements, v)
	// }

	return allSorts, nil
}

// // GetSortReplacements will decode passed paramName paramter and decode it from FormRequest into []*Sort
// // It will then apply these sorts to passed query and return extracted values
// // Will apply "order by" text to query if not found
// // Throws SortError{} error type if error occurs
// func GetSortReplacements(
// 	r FormRequest,
// 	query *string,
// 	paramName string,
// 	prependSorts []*Sort,
// 	fields map[string]FieldConfig,
// ) ([]*Sort, []interface{}, error) {
// 	var sortSlice []*Sort
// 	var replacements []interface{}
// 	var err error

// 	orderExp := regexp.MustCompile(`(?i)(\n|\t|\s)order(\n|\t|\s)`)

// 	// if s := orderExp.FindString(*query); s == "" {
// 	// 	*query += " order by"
// 	// } else {
// 	// 	*query = ","
// 	// }

// 	// for _, v := range prependSorts {

// 	// }

// 	if sortSlice, err = DecodeSorts(r, paramName); err != nil {
// 		return nil, nil, err
// 	}

// 	if len(sortSlice) > 0 {
// 		if s := orderExp.FindString(*query); s == "" {
// 			*query += " order by"
// 		} else {
// 			*query = ","
// 		}

// 		if replacements, err = ReplaceSortFields(query, sortSlice, fields); err != nil {
// 			return nil, nil, err
// 		}
// 	} else {
// 		replacements = make([]interface{}, 0)
// 	}

// 	return nil, replacements, nil
// }

// GetGroupReplacements will decode passed paramName paramter and decode it from FormRequest into []*Group
// It will then apply these groups to passed query and return extracted values
// Will apply "group by" text to query if not found
// Throws GroupError{} error type if error occurs
func GetGroupReplacements(
	r FormRequest,
	query *string,
	paramName string,
	queryConf QueryConfig,
	// excludeGroups bool,
	// prependGroups []Group,
	fields map[string]FieldConfig,
) ([]Group, error) {
	var allGroups, groupSlice []Group
	//var replacements, prependReplacements []interface{}
	var err error

	groupExp := regexp.MustCompile(`(?i)(\n|\t|\s)group(\n|\t|\s)`)

	if queryConf.PrependGroupFields != nil {
		if len(queryConf.PrependGroupFields) > 0 {
			if g := groupExp.FindString(*query); g == "" {
				*query += " group by "
			} else {
				*query += ","
			}

			if err = ReplaceGroupFields(
				query,
				queryConf.PrependGroupFields,
				fields,
			); err != nil {
				return nil, errors.Wrap(err, "")
			}
		}
	} else {
		queryConf.PrependGroupFields = make([]Group, 0)
	}

	if !queryConf.ExcludeGroups {
		if groupSlice, err = DecodeGroups(r, paramName); err != nil {
			return nil, errors.Wrap(err, "")
		}

		if len(groupSlice) > 0 {
			if g := groupExp.FindString(*query); g == "" {
				*query += " group by "
			} else {
				*query += ","
			}

			if err = ReplaceGroupFields(query, groupSlice, fields); err != nil {
				return nil, errors.Wrap(err, "")
			}
		}
	} else {
		groupSlice = make([]Group, 0)
	}

	allGroups = make([]Group, 0, len(queryConf.PrependGroupFields)+len(groupSlice))
	//allReplacements = make([]interface{}, 0, len(prependReplacements)+len(replacements))

	for _, v := range queryConf.PrependGroupFields {
		allGroups = append(allGroups, v)
	}
	for _, v := range groupSlice {
		allGroups = append(allGroups, v)
	}

	// for _, v := range prependReplacements {
	// 	allReplacements = append(allReplacements, v)
	// }
	// for _, v := range replacements {
	// 	allReplacements = append(allReplacements, v)
	// }

	return allGroups, nil
}

// // GetGroupReplacements will decode passed paramName paramter and decode it from FormRequest into []*Group
// // It will then apply these groups to passed query and return extracted values
// // Will apply "group by" text to query if not found
// // Throws GroupError{} error type if error occurs
// func GetGroupReplacements(
// 	r FormRequest,
// 	query *string,
// 	paramName string,
// 	prependGroups []*Group,
// 	fields map[string]FieldConfig,
// ) ([]*Group, []interface{}, error) {
// 	var groupSlice []*Group
// 	var replacements []interface{}
// 	var err error

// 	if groupSlice, err = DecodeGroups(r, paramName); err != nil {
// 		return nil, nil, err
// 	}

// 	if len(groupSlice) > 0 {
// 		groupExp := regexp.MustCompile(`(?i)(\n|\t|\s)group(\n|\t|\s)`)

// 		if s := groupExp.FindString(*query); s == "" {
// 			*query += " group by"
// 		} else {
// 			*query += ","
// 		}

// 		if replacements, err = ReplaceGroupFields(query, groupSlice, fields); err != nil {
// 			return nil, nil, err
// 		}
// 	} else {
// 		replacements = make([]interface{}, 0)
// 	}

// 	return nil, replacements, nil
// }

func GetLimitWithOffsetReplacements(
	r FormRequest,
	query *string,
	takeParam,
	skipParam string,
	takeLimit int,
) ([]interface{}, error) {
	var err error
	var takeInt, skipInt int

	take := r.FormValue(takeParam)
	skip := r.FormValue(skipParam)

	if take == "" {
		takeInt = takeLimit
	} else {
		if takeInt, err = strconv.Atoi(take); err != nil {
			return nil, errors.Wrap(err, "")
		}

		if takeInt > takeLimit {
			takeInt = takeLimit
		}
	}

	if skip == "" {
		skipInt = 0
	} else {
		if skipInt, err = strconv.Atoi(skip); err != nil {
			return nil, errors.Wrap(err, "")
		}
	}

	replacements := []interface{}{takeInt, skipInt}
	ApplyLimit(query)
	return replacements, nil
}

////////////////////////////////////////////////////////////
// DECODE FUNCTIONS
////////////////////////////////////////////////////////////

func decodeQueryParams(r FormRequest, paramName string, val interface{}) error {
	formVal := r.FormValue(paramName)

	if formVal != "" {
		param, err := url.QueryUnescape(formVal)

		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(param), &val)

		if err != nil {
			return errors.Wrap(err, "")
		}
	}

	return nil
}

// DecodeFilters will use passed paramName parameter to extract json encoded
// filter from passed FormRequest and decode into Filter
// If paramName is not found in FormRequest, error will be thrown
// Will also throw error if can't properly decode
func DecodeFilters(r FormRequest, paramName string) ([]Filter, error) {
	var filterArray []Filter
	var err error

	if err = decodeQueryParams(r, paramName, &filterArray); err != nil {
		return nil, errors.Wrap(err, "")
	}

	return filterArray, nil
}

// DecodeSorts will use passed paramName parameter to extract json encoded
// sort from passed FormRequest and decode into Sort
// If paramName is not found in FormRequest, error will be thrown
// Will also throw error if can't properly decode
func DecodeSorts(r FormRequest, paramName string) ([]Sort, error) {
	var sortArray []Sort
	var err error

	if err = decodeQueryParams(r, paramName, &sortArray); err != nil {
		return nil, errors.Wrap(err, "")
	}

	return sortArray, nil
}

func DecodeGroups(r FormRequest, paramName string) ([]Group, error) {
	var groupSlice []Group
	var err error

	if err = decodeQueryParams(r, paramName, &groupSlice); err != nil {
		return nil, err
	}

	return groupSlice, nil
}

////////////////////////////////////////////////////////////
// REPLACE FUNCTIONS
////////////////////////////////////////////////////////////

// ReplaceFilterFields is used to replace query field names and values from slice of filters
// along with verifying that they have right values and applying changes to query
// This function does not apply "where" string for query so one must do it before
// passing query
func ReplaceFilterFields(query *string, filters []Filter, fields map[string]FieldConfig) ([]interface{}, error) {
	var err error
	replacements := make([]interface{}, 0, len(filters))

	for i, v := range filters {
		var r interface{}
		containsField := false

		// Check if current filter is within our fields map
		// If it is, check that it is allowed to be filtered
		// by and then check if given parameters are valid
		// If valid, apply filter to query
		// Else throw error
		if conf, ok := fields[v.Field]; ok {
			if !conf.OperationConf.CanFilterBy {
				filterErr := &FilterError{}
				filterErr.setInvalidFilterError(conf.DBField)
				return nil, errors.Wrap(filterErr, "")
			}

			//replacements = append(replacements, conf.DBField)
			containsField = true

			if r, err = FilterCheck(v); err != nil {
				return nil, errors.Wrap(err, "")
			}

			replacements = append(replacements, r)

			applyAnd := true

			if i == len(filters)-1 {
				applyAnd = false
			}

			v.Field = conf.DBField
			ApplyFilter(query, v, applyAnd)
		}

		if !containsField {
			filterErr := &FilterError{}
			filterErr.setInvalidFilterError(v.Field)
			return nil, errors.Wrap(filterErr, "")
		}
	}

	return replacements, nil
}

// ReplaceSortFields is used to replace query field names and values from slice of sorts
// along with verifying that they have right values and applying changes to query
// This function does not apply "order by" string for query so one must do it before
// passing query
func ReplaceSortFields(query *string, sorts []Sort, fields map[string]FieldConfig) error {
	var err error
	replacements := make([]interface{}, 0, len(sorts))

	for i, v := range sorts {
		containsField := false

		// Check if current sort is within our fields map
		// If it is, check that it is allowed to be sorted
		// by and then check if given parameters are valid
		// If valid, apply sort to query
		// Else throw error
		if conf, ok := fields[v.Field]; ok {
			if !conf.OperationConf.CanSortBy {
				sortErr := &SortError{}
				sortErr.setInvalidSortError(v.Field)
				return errors.Wrap(sortErr, "")
			}

			if err = SortCheck(v, replacements); err != nil {
				return err
			}

			addComma := true

			if i == len(sorts)-1 {
				addComma = false
			}

			v.Field = conf.DBField
			ApplySort(query, v, addComma)
			containsField = true
		}

		if !containsField {
			sortErr := &SortError{}
			sortErr.setInvalidSortError(v.Field)
			return errors.Wrap(sortErr, "")
		}
	}

	return nil
}

func ReplaceGroupFields(query *string, groups []Group, fields map[string]FieldConfig) error {
	//replacements := make([]interface{}, 0, len(groups))

	for i, v := range groups {
		containsField := false

		// Check if current sort is within our fields map
		// If it is, check that it is allowed to be grouped
		// by and then check if given parameters are valid
		// If valid, apply sort to query
		// Else throw error
		if conf, ok := fields[v.Field]; ok {
			if !conf.OperationConf.CanGroupBy {
				groupErr := &GroupError{}
				groupErr.setInvalidGroupError(v.Field)
				return errors.Wrap(groupErr, "")
			}

			addComma := true

			if i == len(groups)-1 {
				addComma = false
			}

			v.Field = conf.DBField
			ApplyGroup(query, v, addComma)
			containsField = true
		}

		if !containsField {
			groupErr := &GroupError{}
			groupErr.setInvalidGroupError(v.Field)
			return errors.Wrap(groupErr, "")
		}
	}

	return nil
}

////////////////////////////////////////////////////////////
// APPLY FUNCTIONS
////////////////////////////////////////////////////////////

// ApplyFilter applies the filter passed to the query passed
// The applyAnd paramter is used to determine if the query should have
// an "and" added to the end
func ApplyFilter(query *string, filter Filter, applyAnd bool) {
	_, ok := filter.Value.([]interface{})

	if ok {
		*query += " " + filter.Field + " in (?)"
	} else {
		switch filter.Operator {
		case "eq":
			*query += " " + filter.Field + " = ?"
		case "neq":
			*query += " " + filter.Field + " != ?"
		case "startswith":
			*query += " " + filter.Field + " ilike ? || '%'"
		case "endswith":
			*query += " " + filter.Field + " ilike '%' || ?"
		case "contains":
			*query += " " + filter.Field + " ilike '%' || ? || '%'"
		case "doesnotcontain":
			*query += " " + filter.Field + " not ilike '%' || ? || '%'"
		case "isnull":
			*query += " " + filter.Field + " is null"
		case "isnotnull":
			*query += " " + filter.Field + " is not null"
		case "isempty":
			*query += " " + filter.Field + " = ''"
		case "isnotempty":
			*query += " " + filter.Field + " != ''"
		case "lt":
			*query += " " + filter.Field + " < ?"
		case "lte":
			*query += " " + filter.Field + " <= ?"
		case "gt":
			*query += " " + filter.Field + " > ?"
		case "gte":
			*query += " " + filter.Field + " >= ?"
		}
	}

	// If there is more in filter slice, append "and"
	if applyAnd {
		*query += " and"
	}
}

// ApplySort applies the sort passed to the query passed
// The addComma paramter is used to determine if the query should have
// ","(comma) appended to the query
func ApplySort(query *string, sort Sort, addComma bool) {
	*query += " " + sort.Field

	if sort.Dir == "asc" {
		*query += " asc"
	} else {
		*query += " desc"
	}

	if addComma {
		*query += ","
	}
}

func ApplyGroup(query *string, group Group, addComma bool) {
	*query += " " + group.Field

	if addComma {
		*query += ","
	}
}

////////////////////////////////////////////////////////////
// CHECK FUNCTIONS
////////////////////////////////////////////////////////////

// SortCheck checks to make sure that the "dir" field either has value "asc" or "desc"
// and if it doesn't, throw error
// It also adds the field to the replacements parameter passed
func SortCheck(s Sort, replacements []interface{}) error {
	if s.Dir != "asc" && s.Dir != "desc" {
		sortErr := &SortError{}
		sortErr.setInvalidDirError(s.Field, s.Dir)
		return sortErr
	}

	return nil

	// replacements = append(replacements, s.Field)
	// return nil
}

// FilterCheck checks to make sure that the values passed to each filter is valid
// The types passed should be primitive types
// It also adds the field to the replacements parameter passed
func FilterCheck(f Filter) (interface{}, error) {
	var r interface{}

	validTypes := []string{"string", "float64", "int64"}
	hasValidType := false

	if f.Value != "" && f.Operator != "isnull" && f.Operator != "isnotnull" {
		// First check if value sent is slice
		list, ok := f.Value.([]interface{})

		// If slice, then loop through and make sure all items in list
		// are primitive type, else throw error
		if ok {
			for _, t := range list {
				someType := reflect.TypeOf(t)

				for _, v := range validTypes {
					if someType.String() == v {
						hasValidType = true
						break
					}
				}

				if !hasValidType {
					sliceErr := &SliceError{}
					sliceErr.setInvalidSliceError(f.Field, someType.String())
					return nil, sliceErr
				}
			}

			r = list
		} else {
			validTypes = append(validTypes, "bool")

			if f.Value == nil {
				filterErr := &FilterError{}
				filterErr.setInvalidValueError(f.Field, f.Value)
				return nil, filterErr
			}

			someType := reflect.TypeOf(f.Value)

			for _, v := range validTypes {
				if someType.String() == v {
					hasValidType = true
					break
				}
			}

			if !hasValidType {
				sliceErr := &SliceError{}
				sliceErr.setInvalidSliceError(f.Field, someType.String())
				return nil, sliceErr
			}

			r = f.Value
		}
	}

	return r, nil
}

// --------------------------------------------------------------------------------

/////////////////////////////////////////////
// DECODE LOGIC
/////////////////////////////////////////////

// DecodeSort takes an encoded url string, unescapes it and then
// unmarshals it to return a *Sort struct
func DecodeSort(sortEncoding string) (*Sort, error) {
	var sort *Sort
	param, err := url.QueryUnescape(sortEncoding)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(param), &sort)

	if err != nil {
		return nil, err
	}

	return sort, nil
}

// DecodeFilter takes encoded url string, unescapes it and then
// unmarshals it to return a []*Filter slice
func DecodeFilter(filterEncoding string) ([]*Filter, error) {
	var filterArray []*Filter
	param, err := url.QueryUnescape(filterEncoding)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(param), &filterArray)

	if err != nil {
		return nil, err
	}

	return filterArray, nil
}

/////////////////////////////////////////////
// FILTER LOGIC
/////////////////////////////////////////////

func applyFilters(query *string, filters []*Filter) {
	if len(filters) > 0 {
		var selectCount int
		var whereCount int

		// Regular expression for checking whether the given query
		// already has a where statement
		selectExp := regexp.MustCompile(`(?i)(\n|\t|\s|\A)select(\n|\t|\s)`)
		whereExp := regexp.MustCompile(`(?i)(\n|\t|\s)where(\n|\t|\s)`)

		selectSlice := selectExp.FindAllStringIndex(*query, -1)
		whereSlice := whereExp.FindAllStringIndex(*query, -1)

		if selectSlice != nil {
			selectCount = len(selectSlice)
		}
		if whereSlice != nil {
			whereCount = len(whereSlice)
		}

		if whereCount < selectCount {
			*query += " where "
		} else {
			*query += " and "
		}

		// Loop through given filters and apply search criteria to query
		// based off of filter operator
		for i := 0; i < len(filters); i++ {
			_, ok := filters[i].Value.([]interface{})

			if ok {
				*query += " " + filters[i].Field + " in (?)"
			} else {
				switch filters[i].Operator {
				case "eq":
					*query += " " + filters[i].Field + " = ?"
				case "neq":
					*query += " " + filters[i].Field + " != ?"
				case "startswith":
					*query += " " + filters[i].Field + " ilike ? || '%'"
				case "endswith":
					*query += " " + filters[i].Field + " ilike '%' || ?"
				case "contains":
					*query += " " + filters[i].Field + " ilike '%' || ? || '%'"
				case "doesnotcontain":
					*query += " " + filters[i].Field + " not ilike '%' || ? || '%'"
				case "isnull":
					*query += " " + filters[i].Field + " is null"
				case "isnotnull":
					*query += " " + filters[i].Field + " is not null"
				case "isempty":
					*query += " " + filters[i].Field + " = ''"
				case "isnotempty":
					*query += " " + filters[i].Field + " != ''"
				case "lt":
					*query += " " + filters[i].Field + " < ?"
				case "lte":
					*query += " " + filters[i].Field + " <= ?"
				case "gt":
					*query += " " + filters[i].Field + " > ?"
				case "gte":
					*query += " " + filters[i].Field + " >= ?"
				}
			}

			// If there is more in filter slice, append "and"
			if i != len(filters)-1 {
				*query += " and"
			}
		}
	}
}

func ApplyFilterV2(query *string, filters []*Filter, exclusionFields []string) {
	for i, v := range filters {
		for _, t := range exclusionFields {
			if v.Field == t {
				filters = append(filters[:i], filters[i+1:]...)
			}
		}
	}

	applyFilters(query, filters)
}

// ApplyFilters takes a query string with a slice of Filter structs
// and applies where filtering to the query
func ApplyFilters(query *string, filters []*Filter) {
	applyFilters(query, filters)
}

// -------------------------------------------------------------------------------

// ApplyLimit takes given query and applies limit and offset criteria
func ApplyLimit(query *string) {
	*query += " limit ? offset ?"
}

// ApplyOrdering takes given query and applies the given sort criteria
func ApplyOrdering(query *string, sort *Sort) {
	*query += " order by " + snaker.CamelToSnake(sort.Field) + " " + sort.Dir
}

/////////////////////////////////////////////
// WHERE FILTER LOGIC
/////////////////////////////////////////////

func whereFilter(
	r FormRequest,
	query *string,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
	fieldNamesV2 map[string]string,
	exclusionFields []string,
) ([]interface{}, error) {
	var err error
	varReplacements := make([]interface{}, 0)
	filtersEncoded := r.FormValue("filters")

	if prependVars != nil {
		varReplacements = append(varReplacements, prependVars...)
	}

	if filtersEncoded != "" {
		var replacements []interface{}

		filters, err := DecodeFilter(filtersEncoded)

		if err != nil {
			return nil, err
		}

		if fieldNames != nil {
			replacements, err = replaceFields(filters, fieldNames)
		} else {
			replacements, err = replaceFieldsV2(filters, fieldNamesV2)
		}

		if err != nil {
			return nil, err
		}

		if exclusionFields == nil {
			ApplyFilters(query, filters)
		} else {
			ApplyFilterV2(query, filters, exclusionFields)
		}
		varReplacements = append(varReplacements, replacements...)
	}

	*query, varReplacements, err = InQueryRebind(bindVar, *query, varReplacements...)

	if err != nil {
		return nil, err
	}

	// *query = sqlx.Rebind(bindVar, *query)

	// *query, varReplacements, err = sqlx.In(*query, varReplacements...)

	// if err != nil {
	// 	return nil, err
	// }

	// *query = sqlx.Rebind(bindVar, *query)

	return varReplacements, nil
}

// WhereFilter decodes all the given values from the passed FormRequest
// and applies it to given query
// This function simply applies other functions like
// DecodeFilter, ApplyFilters
// This function is meant to be used for aggregate queries that
// need server side filtering
//
// r:
// 		Struct that implements "FormValue(string)string", which will
// 		generally be http.Request
// query:
// 		The query to be modified
// bindVar:
// 		The binding var used for query eg. sql.DOLLAR
// prependVars:
// 		Slice of values that should be used that do not apply
// 		to modified query.  See example for better explanation
// fieldNames:
// 		Slice of field names that the filter can apply to
// 		These field names should be the name of database fields.
// 		Reason for this is to avoid sql injection as field names
// 		can't be used a placeholders like values can in a query
// 		so if any given filter name does not match any of the field
// 		names in the slice, then an error will be thrown
func WhereFilter(
	r FormRequest,
	query *string,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
) ([]interface{}, error) {
	return whereFilter(r, query, bindVar, prependVars, fieldNames, nil, nil)
}

// WhereFilterV2 decodes all the given values from the passed FormRequest
// and applies it to given query
// This function simply applies other functions like
// DecodeFilter, ApplyFilters
// This function is meant to be used for aggregate queries that
// need server side filtering
// WhereFilterV2 adds exclusionFields parameter to original WhereFilter function
// Reason for new parameter is that there are situations where we may want to exclude
// some filters from being passed to our filters.  See example for explanation
//
// r:
// 		Struct that implements "FormValue(string)string", which will
// 		generally be http.Request
// query:
// 		The query to be modified
// bindVar:
// 		The binding var used for query eg. sql.DOLLAR
// prependVars:
// 		Slice of values that should be used that do not apply
// 		to modified query.  See example for better explanation
// fieldNames:
// 		Slice of field names that the filter can apply to
// 		These field names should be the name of database fields.
// 		Reason for this is to avoid sql injection as field names
// 		can't be used a placeholders like values can in a query
// 		so if any given filter name does not match any of the field
// 		names in the slice, then an error will be thrown
// exclusionFields:
//		Fields to exclude from form filters
//
func WhereFilterV2(
	r FormRequest,
	query *string,
	bindVar int,
	prependVars []interface{},
	fieldNames map[string]string,
	exclusionFields []string,
) ([]interface{}, error) {
	return whereFilter(r, query, bindVar, prependVars, nil, fieldNames, exclusionFields)
}

/////////////////////////////////////////////
// APPLY ALL LOGIC
/////////////////////////////////////////////

func applyAll(
	r FormRequest,
	query *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
	fieldNamesV2 map[string]string,
	applyConfig *ApplyConfig,
) ([]interface{}, error) {
	var err error
	var intTake uint64

	filters := make([]*Filter, 0)
	varReplacements := make([]interface{}, 0)
	take := r.FormValue("take")
	skip := r.FormValue("skip")
	filtersEncoded := r.FormValue("filters")
	sortEncoded := r.FormValue("sort")

	if take == "" {
		take = "0"
		intTake = uint64(0)
	} else {
		intTake, err = strconv.ParseUint(take, 10, 32)

		if err != nil {
			return nil, errors.New(err.Error())
		}
	}

	if skip == "" {
		skip = "0"
	}

	if intTake > takeLimit && takeLimit > 0 {
		take = strconv.FormatUint(takeLimit, 10)
	}

	if prependVars != nil {
		varReplacements = append(varReplacements, prependVars...)
	}

	if filtersEncoded != "" {
		var replacements []interface{}

		filters, err = DecodeFilter(filtersEncoded)

		if err != nil {
			return nil, err
		}

		if fieldNames != nil {
			replacements, err = replaceFields(filters, fieldNames)
		} else {
			replacements, err = replaceFieldsV2(filters, fieldNamesV2)
		}

		if err != nil {
			return nil, err
		}

		if applyConfig != nil {
			if applyConfig.ExclusionFields == nil {
				ApplyFilters(query, filters)
			} else {
				ApplyFilterV2(query, filters, applyConfig.ExclusionFields)
			}
		} else {
			ApplyFilters(query, filters)
		}

		varReplacements = append(varReplacements, replacements...)
	}

	if sortEncoded != "" {
		sort, err := DecodeSort(sortEncoded)

		if err != nil {
			return nil, err
		}

		if sort.Dir != "asc" && sort.Dir != "desc" {
			return nil, ErrInvalidSort
		}

		if fieldNamesV2 != nil {
			if _, ok := fieldNamesV2[sort.Field]; !ok {
				fmt.Printf("sort field: %s\n", sort.Field)
				filterErr := &FilterError{}
				filterErr.setInvalidFilterError(sort.Field)
				return nil, filterErr
			}

			sort.Field = fieldNamesV2[sort.Field]
		}

		if applyConfig != nil {
			if applyConfig.ApplyOrdering {
				ApplyOrdering(query, sort)
			}
		} else {
			ApplyOrdering(query, sort)
		}
	}

	if applyConfig != nil {
		if applyConfig.ApplyLimit {
			varReplacements = append(varReplacements, take, skip)
			ApplyLimit(query)
		}
	} else {
		varReplacements = append(varReplacements, take, skip)
		ApplyLimit(query)
	}

	*query, varReplacements, err = InQueryRebind(bindVar, *query, varReplacements...)

	if err != nil {
		return nil, err
	}

	return varReplacements, nil
}

// ApplyAll is the main function that will be used for server side filtering
// It applies most of the other functions written including DecodeFilter, ApplyFilters,
// DecodeSort, ApplyOrdering and ApplyLimit
//
// r:
// 		Struct that implements "FormValue(string)string", which will
// 		generally be http.Request
// query:
// 		The query to be modified
// takeLimit:
// 		Applies limit to the number of returned rows
// 		If 0, no limit is set
// bindVar:
// 		The binding var used for query eg. sql.DOLLAR
// prependVars:
// 		Slice of values that should be used that do not apply
// 		to modified query.  See example for better explanation
// fieldNames:
// 		Slice of field names that the filter can apply to
// 		These field names should be the name of database fields.
// 		Reason for this is to avoid sql injection as field names
// 		can't be used as placeholders like values can in a query
// 		so if any given filter name does not match any of the field
// 		names in the slice, then an error will be thrown
func ApplyAll(
	r FormRequest,
	query *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
) ([]interface{}, error) {
	return applyAll(r, query, takeLimit, bindVar, prependVars, fieldNames, nil, nil)
}

// ApplyAllV2 is the main function that will be used for server side filtering
// It applies most of the other functions written including DecodeFilter, ApplyFilters,
// DecodeSort, ApplyOrdering and ApplyLimit
//
// r:
// 		Struct that implements "FormValue(string)string", which will
// 		generally be http.Request
// query:
// 		The query to be modified
// takeLimit:
// 		Applies limit to the number of returned rows
// 		If 0, no limit is set
// bindVar:
// 		The binding var used for query eg. sql.DOLLAR
// prependVars:
// 		Slice of values that should be used that do not apply
// 		to modified query.  See example for better explanation
// fieldNames:
// 		Slice of field names that the filter can apply to
// 		These field names should be the name of database fields.
// 		Reason for this is to avoid sql injection as field names
// 		can't be used as placeholders like values can in a query
// 		so if any given filter name does not match any of the field
// 		names in the slice, then an error will be thrown
// applyConfig:
//		Configuration to determine whether to apply certain filters
//
func ApplyAllV2(
	r FormRequest,
	query *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames map[string]string,
	applyconfig *ApplyConfig,
) ([]interface{}, error) {
	return applyAll(r, query, takeLimit, bindVar, prependVars, nil, fieldNames, applyconfig)
}

/////////////////////////////////////////////
// FILTERED RESULTS LOGIC
/////////////////////////////////////////////

// GetFilteredResults is a wrapper function for getting a filtered query from
// ApplyAll function along with getting a count
func GetFilteredResults(
	r FormRequest,
	query *string,
	countQuery *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
	db httputil.DBInterface,
) (httputil.Rower, int, error) {
	replacements, err := ApplyAll(
		r,
		query,
		takeLimit,
		bindVar,
		prependVars,
		fieldNames,
	)

	if err != nil {
		return nil, 0, err
	}

	*query += ";"
	results, err := db.Query(
		*query,
		replacements...,
	)

	if err != nil {
		return nil, 0, err
	}

	countReplacements, err := WhereFilter(
		r,
		countQuery,
		bindVar,
		prependVars,
		fieldNames,
	)

	if err != nil {
		return nil, 0, err
	}

	*countQuery += ";"
	countResults, err := dbutil.QueryCount(
		db,
		*countQuery,
		countReplacements...,
	)

	if err != nil {
		return nil, 0, err
	}

	return results, countResults.Total, nil
}

func GetFilteredResultsV2(
	r FormRequest,
	query *string,
	countQuery *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames map[string]string,
	applyConfig *ApplyConfig,
	db httputil.DBInterface,
) (httputil.Rower, int, []interface{}, []interface{}, error) {
	var rower httputil.Rower
	var count int

	replacements, err := ApplyAllV2(
		r,
		query,
		takeLimit,
		bindVar,
		prependVars,
		fieldNames,
		applyConfig,
	)

	if err != nil {
		confutil.CheckError(err, "")
		fmt.Printf("error 1")
		return nil, 0, nil, nil, err
	}

	executeQuery := func() error {
		*query += ";"
		results, err := db.Query(
			*query,
			replacements...,
		)

		if err != nil {
			fmt.Printf("error 2")
			return err
		}

		rower = results

		return nil
	}

	if applyConfig != nil {
		if applyConfig.ExecuteQuery {
			err = executeQuery()

			if err != nil {
				fmt.Printf("error 3")
				return nil, 0, nil, nil, err
			}
		}
	} else {
		err = executeQuery()

		if err != nil {
			fmt.Printf("error 4")
			return nil, 0, nil, nil, err
		}
	}

	var exclusionFields []string

	if applyConfig != nil {
		exclusionFields = applyConfig.ExclusionFields
	}

	countReplacements, err := WhereFilterV2(
		r,
		countQuery,
		bindVar,
		prependVars,
		fieldNames,
		exclusionFields,
	)

	if err != nil {
		fmt.Printf("error 5")
		return nil, 0, nil, nil, err
	}

	// fmt.Printf("some count repalcements: %v\n", countReplacements)

	executeCountQuery := func() error {
		*countQuery += ";"
		countResults, err := dbutil.QueryCount(
			db,
			*countQuery,
			countReplacements...,
		)

		if err != nil {
			fmt.Printf("error 6")
			return err
		}

		count = countResults.Total

		return nil
	}

	if applyConfig != nil {
		if applyConfig.ExecuteCountQuery {
			err = executeCountQuery()

			if err != nil {
				fmt.Printf("error 7")
				return nil, 0, nil, nil, err
			}
		}
	} else {
		err = executeCountQuery()

		if err != nil {
			fmt.Printf("error 8")
			return nil, 0, nil, nil, err
		}
	}

	return rower, count, replacements, countReplacements, nil
}

// CountSelect take column string and applies count select
func CountSelect(column string) string {
	return fmt.Sprintf("count(%s) as total", column)
}

type GeneralJSON map[string]interface{}

func (g GeneralJSON) Value() (driver.Value, error) {
	j, err := json.Marshal(g)
	return j, err
}

func (g *GeneralJSON) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("Type assertion .([]byte) failed.")
	}

	var i interface{}
	err := json.Unmarshal(source, &i)
	if err != nil {
		return err
	}

	*g, ok = i.(map[string]interface{})
	if !ok {
		arr, ok := i.([]interface{})

		if ok {
			newV := make(map[string]interface{})
			newV["array"] = arr
			*g = newV
		} else {
			return errors.New("Not valid json")
		}
	}

	return nil
}

func InQueryRebind(bindType int, query string, args ...interface{}) (string, []interface{}, error) {
	query, args, err := sqlx.In(query, args...)

	if err != nil {
		return query, nil, err
	}

	query = sqlx.Rebind(bindType, query)
	return query, args, nil
}

func SetRowerResults(
	rower httputil.Rower,
	cache cacheutil.CacheStore,
	cacheSetup cacheutil.CacheSetup,
) error {
	var err error
	columns, err := rower.Columns()

	if err != nil {
		return err
	}

	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	rows := make([]interface{}, 0)
	forms := make([]httputil.FormSelection, 0)

	for rower.Next() {
		form := httputil.FormSelection{}

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		err = rower.Scan(valuePtrs...)

		if err != nil {
			return err
		}

		row := make(map[string]interface{}, 0)
		var idVal interface{}

		for i, k := range columns {
			var v interface{}
			//var formVal string

			val := values[i]

			if k == "id" {
				idVal = val
			}

			switch val.(type) {
			case int64:
				v = strconv.FormatInt(val.(int64), confutil.IntBase)
			case *int64:
				t := val.(*int64)
				if t != nil {
					v = strconv.FormatInt(*t, confutil.IntBase)
				}
			case []byte:
				t := val.([]byte)
				v, err = strconv.ParseFloat(string(t), confutil.IntBitSize)
				if err != nil {
					panic(err)
				}
			default:
				v = val
			}

			var columnName string

			if snaker.IsInitialism(columns[i]) {
				columnName = strings.ToLower(columns[i])
			} else {
				camelCaseJSON := snaker.SnakeToCamelJSON(columns[i])
				firstLetter := strings.ToLower(string(camelCaseJSON[0]))
				columnName = firstLetter + camelCaseJSON[1:]
			}

			row[columnName] = v

			if cacheSetup.FormSelectionConf.ValueColumn == columnName {
				form.Value = v
			}

			if cacheSetup.FormSelectionConf.TextColumn == columnName {
				form.Text = v
			}
		}

		rowBytes, err := json.Marshal(&row)

		if err != nil {
			return err
		}

		var cacheID string

		switch idVal.(type) {
		case int64:
			cacheID = strconv.FormatInt(idVal.(int64), confutil.IntBase)
		case int:
			cacheID = strconv.Itoa(idVal.(int))
		default:
			return errors.New("Invalid id type")
		}

		cache.Set(
			fmt.Sprintf(cacheSetup.CacheIDKey, cacheID),
			rowBytes,
			0,
		)

		rows = append(rows, row)
		forms = append(forms, form)
	}

	rowsBytes, err := json.Marshal(&rows)

	if err != nil {
		return err
	}

	formBytes, err := json.Marshal(&forms)

	if err != nil {
		return err
	}

	cache.Set(cacheSetup.CacheListKey, rowsBytes, 0)
	cache.Set(cacheSetup.FormSelectionConf.FormSelectionKey, formBytes, 0)
	return nil
}

func HasFilterError(w http.ResponseWriter, err error) bool {
	switch err.(type) {
	case *FilterError, *SortError, *GroupError:
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		return true
	}

	return false
}

func replaceFields(filters []*Filter, fieldNames []string) ([]interface{}, error) {
	replacements := make([]interface{}, 0)
	for i, v := range filters {
		containsField := false
		filters[i].Field = snaker.CamelToSnake(filters[i].Field)
		for _, k := range fieldNames {
			if v.Field == k {
				containsField = true

				if v.Value != "" && v.Operator != "isnull" && v.Operator != "isnotnull" {
					list, ok := v.Value.([]interface{})

					if ok {
						for _, t := range list {
							someType := reflect.TypeOf(t)

							if someType.String() == "string" || someType.String() == "float64" || someType.String() == "bool" {
								replacements = append(replacements, t)
							} else {
								return nil, ErrInvalidArray
							}
						}
					} else {
						someType := reflect.TypeOf(v.Value)

						if someType.String() == "string" || someType.String() == "float64" || someType.String() == "bool" {
							replacements = append(replacements, v.Value)
						} else {
							return nil, ErrInvalidValue
						}
					}
				}

				break
			}
		}

		if !containsField {
			filterErr := &FilterError{}
			filterErr.setInvalidFilterError(v.Field)
			return nil, filterErr
		}
	}

	return replacements, nil
}

func replaceFieldsV2(filters []*Filter, fieldNames map[string]string) ([]interface{}, error) {
	var err error
	replacements := make([]interface{}, 0)
	for i, v := range filters {
		containsField := false

		if val, ok := fieldNames[v.Field]; ok {
			filters[i].Field = val
			containsField = true

			replacements, err = filterCheckV1(v, replacements)

			if err != nil {
				return nil, err
			}
		}

		if !containsField {
			filterErr := &FilterError{}
			filterErr.setInvalidFilterError(v.Field)
			return nil, filterErr
		}
	}

	return replacements, nil
}

func filterCheckV1(f *Filter, replacements []interface{}) ([]interface{}, error) {
	if f.Value != "" && f.Operator != "isnull" && f.Operator != "isnotnull" {
		// First check if value sent is slice
		list, ok := f.Value.([]interface{})

		// If slice, then loop through and make sure all items in list
		// are primitive type, else throw error
		if ok {
			for _, t := range list {
				someType := reflect.TypeOf(t)

				if someType.String() != "string" && someType.String() != "float64" {
					sliceErr := &SliceError{}
					sliceErr.setInvalidSliceError(f.Field, someType.String())
					return nil, sliceErr
				}
			}

			replacements = append(replacements, list)
		} else {
			if f.Value == nil {
				filterErr := &FilterError{}
				filterErr.setInvalidValueError(f.Field, f.Value)
				return nil, filterErr
			}

			someType := reflect.TypeOf(f.Value)

			if someType.String() == "string" || someType.String() == "float64" || someType.String() == "bool" {
				replacements = append(replacements, f.Value)
			} else {
				filterErr := &FilterError{}
				filterErr.setInvalidValueError(f.Field, f.Value)
				return nil, filterErr
			}
		}
	}

	return replacements, nil
}
