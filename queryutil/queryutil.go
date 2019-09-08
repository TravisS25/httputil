package queryutil

import (
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
	"github.com/TravisS25/httputil/apiutil"
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

var (
	ErrInvalidSort  = errors.New("invalid sort")
	ErrInvalidArray = errors.New("invalid array for field")
	ErrInvalidValue = errors.New("invalid field value")
)

type FilterError struct {
	field string
	value interface{}
}

func (f *FilterError) Error() string {
	if f.field != "" {
		return fmt.Sprintf("invalid filter: '%s'", f.field)
	}
	if f.value != nil {
		return fmt.Sprintf("invalid value '%v' for filter '%s'", f.value, f.field)
	}

	return ""
}

func (f *FilterError) setInvalidFilterError(field string) {
	f.field = field
}

func (f *FilterError) setInvalidValueError(field string, value interface{}) {
	f.field = field
	f.value = value
}

// FormRequest is used to get form values from url string
// Will mostly come from http.Request
type FormRequest interface {
	FormValue(string) string
}

type ApplyConfig struct {
	ApplyLimit        bool
	ApplyOrdering     bool
	ExecuteQuery      bool
	ExecuteCountQuery bool
	ExclusionFields   []string
}

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

func applyFilter(query *string, filters []*Filter) {
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

	applyFilter(query, filters)
}

// ApplyFilter takes a query string with a slice of Filter structs
// and applies where filtering to the query
func ApplyFilter(query *string, filters []*Filter) {
	applyFilter(query, filters)
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
			ApplyFilter(query, filters)
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
// DecodeFilter, ApplyFilter
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
// DecodeFilter, ApplyFilter
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
				ApplyFilter(query, filters)
			} else {
				ApplyFilterV2(query, filters, applyConfig.ExclusionFields)
			}
		} else {
			ApplyFilter(query, filters)
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
// It applies most of the other functions written including DecodeFilter, ApplyFilter,
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
// It applies most of the other functions written including DecodeFilter, ApplyFilter,
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

// GetFilteredResultsV2 is a wrapper function for getting a filtered query from
// ApplyAllV2 function along with getting a count
// func GetFilteredResultsV2(
// 	r FormRequest,
// 	query *string,
// 	countQuery *string,
// 	takeLimit uint64,
// 	bindVar int,
// 	prependVars []interface{},
// 	fieldNames map[string]string,
// 	applyConfig *ApplyConfig,
// 	db httputil.DBInterface,
// ) (httputil.Rower, int, error) {
// 	var rower httputil.Rower
// 	var count int

// 	replacements, err := ApplyAllV2(
// 		r,
// 		query,
// 		takeLimit,
// 		bindVar,
// 		prependVars,
// 		fieldNames,
// 		applyConfig,
// 	)

// 	if err != nil {
// 		return nil, 0, err
// 	}

// 	if applyConfig != nil {
// 		if applyConfig.ExecuteQuery {
// 			*query += ";"
// 			results, err := db.Query(
// 				*query,
// 				replacements...,
// 			)

// 			if err != nil {
// 				return nil, 0, err
// 			}

// 			rower = results
// 		}
// 	}

// 	var exclusionFields []string

// 	if applyConfig != nil {
// 		exclusionFields = applyConfig.ExclusionFields
// 	}

// 	countReplacements, err := WhereFilterV2(
// 		r,
// 		countQuery,
// 		bindVar,
// 		prependVars,
// 		fieldNames,
// 		exclusionFields,
// 	)

// 	if err != nil {
// 		return nil, 0, err
// 	}

// 	if applyConfig != nil {
// 		if applyConfig.ExecuteCountQuery {
// 			*countQuery += ";"
// 			countResults, err := dbutil.QueryCount(
// 				db,
// 				*countQuery,
// 				countReplacements...,
// 			)

// 			if err != nil {
// 				return nil, 0, err
// 			}

// 			count = countResults.Total
// 		}
// 	}

// 	return rower, count, nil
// }

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

// func GetRowerResults(rower httputil.Rower) ([]interface{}, error) {
// 	var err error
// 	columns, err := rower.Columns()

// 	if err != nil {
// 		return nil, err
// 	}

// 	count := len(columns)
// 	values := make([]interface{}, count)
// 	valuePtrs := make([]interface{}, count)
// 	rows := make([]interface{}, 0)

// 	for rower.Next() {
// 		for i := range columns {
// 			valuePtrs[i] = &values[i]
// 		}

// 		err = rower.Scan(valuePtrs...)

// 		if err != nil {
// 			return nil, err
// 		}

// 		row := make(map[string]interface{}, 0)

// 		for i := range columns {
// 			var v interface{}

// 			val := values[i]

// 			switch val.(type) {
// 			case int64:
// 				v = strconv.FormatInt(val.(int64), apiutil.IntBase)
// 			case float64:
// 				v = strconv.FormatFloat(val.(float64), 'g', 'g', confutil.IntBitSize)
// 			default:
// 				v = val
// 			}

// 			row[columns[i]] = v
// 		}

// 		rows = append(rows, row)
// 	}

// 	return rows, nil
// }

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
				v = strconv.FormatInt(val.(int64), apiutil.IntBase)
				//formVal = strconv.FormatInt(v.(int64), confutil.IntBase)
			case *int64:
				t := val.(*int64)
				if t != nil {
					v = strconv.FormatInt(*t, apiutil.IntBase)
					//formVal = strconv.FormatInt(*t, confutil.IntBase)
				}
			case []byte:
				t := val.([]byte)
				v, err = strconv.ParseFloat(string(t), confutil.IntBitSize)
				if err != nil {
					panic(err)
				}
				//v = f
				//formVal = strconv.FormatFloat(f, 'f', 2, confutil.IntBitSize)
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
	if _, ok := err.(*FilterError); ok {
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

			replacements, err = filterCheck(v, replacements)

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

func filterCheck(f *Filter, replacements []interface{}) ([]interface{}, error) {
	if f.Value != "" && f.Operator != "isnull" && f.Operator != "isnotnull" {
		list, ok := f.Value.([]interface{})

		if ok {
			for _, t := range list {
				someType := reflect.TypeOf(t)

				if someType.String() != "string" && someType.String() != "float64" {
					return nil, ErrInvalidArray
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
