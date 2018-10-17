package queryutil

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/dbutil"
	"github.com/knq/snaker"

	"github.com/jmoiron/sqlx"

	"reflect"

	"github.com/pkg/errors"
)

const (
	// Select string for queries
	Select        = "select "
	invalidFilter = "invalid filter: '%s'"
)

var (
	//ErrQueryNil      = errors.New("query can't be nil")
	//ErrInvalidFilter = errors.New("invalid filter")
	ErrInvalidSort  = errors.New("invalid sort")
	ErrInvalidArray = errors.New("invalid array for field")
	ErrInvalidValue = errors.New("invalid field value")
)

// FormRequest is used to get form values from url string
// Will mostly come from http.Request
type FormRequest interface {
	FormValue(string) string
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

// ApplyFilter takes a query string with a slice of Filter structs
// and applies where filtering to the query
func ApplyFilter(query *string, filters []*Filter) {
	if len(filters) > 0 {
		// Regular expression for checking whether the given query
		// already has a where statement
		re := regexp.MustCompile(`(?i)(\n|\t|\s)where(\n|\t|\s)`)

		// If query already has where statement, apply "and" to the query
		// with the filters
		// Else apply where clause with filters
		if re.MatchString(*query) {
			*query += " and "
		} else {
			*query += " where "
		}

		// Loop through given filters and apply search criteria to query
		// based off of filter operator
		for i := 0; i < len(filters); i++ {
			list, ok := filters[i].Value.([]interface{})

			if ok {
				for _, v := range list {
					someType := reflect.TypeOf(v)

					if someType.String() == "string" || someType.String() == "float64" {
						switch filters[i].Operator {
						case "eq":
							*query += " " + filters[i].Field + " in (?)"
						}
					}
				}
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

// ApplyLimit takes given query and applies limit and offset criteria
func ApplyLimit(query *string) {
	*query += " limit ? offset ?"
}

// ApplyOrdering takes given query and applies the given sort criteria
func ApplyOrdering(query *string, sort *Sort) {
	*query += " order by " + snaker.CamelToSnake(sort.Field) + " " + sort.Dir
}

// WhereFilter decodes all the given values from the passed FormRequest
// and applies it to given query
// This function simply applies other functions like
// DecodeFilter, ApplyFilter
// This function is meant to be used for aggregate queries that
// need server side filtering
//
// r:
// Struct that implements "FormValue(string)string", which will
// generally be http.Request
// query:
// The query to be modified
// bindVar:
// The binding var used for query eg. sql.DOLLAR
// prependVars:
// Slice of values that should be used that do not apply
// to modified query.  See example for better explanation
// fieldNames:
// Slice of field names that the filter can apply to
// These field names should be the name of database fields.
// Reason for this is to avoid sql injection as field names
// can't be used a placeholders like values can in a query
// so if any given filter name does not match any of the field
// names in the slice, then an error will be thrown
func WhereFilter(
	r FormRequest,
	query *string,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
) ([]interface{}, error) {
	varReplacements := make([]interface{}, 0)
	filtersEncoded := r.FormValue("filters")

	if prependVars != nil {
		varReplacements = append(varReplacements, prependVars...)
	}

	if filtersEncoded != "" {
		filters, err := DecodeFilter(filtersEncoded)

		if err != nil {
			return nil, err
		}

		replacements, err := replaceFields(filters, fieldNames)

		if err != nil {
			return nil, err
		}

		ApplyFilter(query, filters)
		varReplacements = append(varReplacements, replacements...)
	}

	newQuery := sqlx.Rebind(bindVar, *query)
	*query = newQuery

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
	var err error
	var intTake uint64
	filters := make([]*Filter, 0)
	varReplacements := make([]interface{}, 0)
	take := r.FormValue("take")
	skip := r.FormValue("skip")
	filtersEncoded := r.FormValue("filters")
	sortEncoded := r.FormValue("sort")

	if take == "" {
		intTake = uint64(10)
	} else {
		intTake, err = strconv.ParseUint(take, 10, 32)

		if err != nil {
			return nil, errors.New(err.Error())
		}
	}

	if skip == "" {
		skip = "0"
	}

	if err != nil {
		return nil, err
	}

	if intTake > takeLimit && takeLimit > 0 {
		take = strconv.FormatUint(takeLimit, 10)
	}

	if prependVars != nil {
		varReplacements = append(varReplacements, prependVars...)
	}

	if filtersEncoded != "" {
		filters, err = DecodeFilter(filtersEncoded)

		if err != nil {
			return nil, err
		}

		replacements, err := replaceFields(filters, fieldNames)

		if err != nil {
			return nil, err
		}

		ApplyFilter(query, filters)
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

		ApplyOrdering(query, sort)
	}

	if intTake != uint64(0) {
		ApplyLimit(query)
		varReplacements = append(varReplacements, intTake, skip)
	}

	newQuery := sqlx.Rebind(bindVar, *query)
	*query = newQuery

	return varReplacements, nil
}

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

		// return errors.New("Type assertion .(map[string]interface{}) failed.")
	}

	return nil
}

func InQueryRebind(bindType int, query string, args ...interface{}) (string, []interface{}, error) {
	query, args, err := sqlx.In(query, args...)

	if err != nil {
		return "", nil, err
	}

	query = sqlx.Rebind(bindType, query)
	return query, args, nil
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

							if someType.String() == "string" || someType.String() == "float64" {
								replacements = append(replacements, t)
							} else {
								return nil, ErrInvalidArray
							}
						}
					} else {
						someType := reflect.TypeOf(v.Value)

						if someType.String() == "string" || someType.String() == "float64" {
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
			err := fmt.Errorf(invalidFilter, v.Field)
			return nil, err
		}
	}

	return replacements, nil
}
