package queryutil

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/knq/snaker"

	"github.com/jmoiron/sqlx"

	"github.com/pkg/errors"
)

const (
	Select        = "select "
	invalidFilter = "invalid filter: '%s'"
)

var (
	ErrQueryNil      = errors.New("query can't be nil")
	ErrInvalidFilter = errors.New("invalid filter")
	ErrInvalidSort   = errors.New("invalid sort")
)

type FormRequest interface {
	FormValue(string) string
}

type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type Sort struct {
	Dir   string `json:"dir"`
	Field string `json:"field"`
}

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

func ApplyFilter(query *string, filters []*Filter) {
	if len(filters) > 0 {
		// *query = strings.ToLower(*query)
		re := regexp.MustCompile(`(?i)(\n|\t|\s)where(\n|\t|\s)`)

		if re.MatchString(*query) {
			*query += " and "
		} else {
			*query += " where "
		}

		for i := 0; i < len(filters); i++ {
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

			if i != len(filters)-1 {
				*query += " and"
			}
		}
	}
}

func ApplyLimit(query *string) {
	*query += " limit ? offset ?"
}

func ApplyOrdering(query *string, sort *Sort) {
	*query += " order by " + snaker.CamelToSnake(sort.Field) + " " + sort.Dir
}

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

func ApplyAll(
	r FormRequest,
	query *string,
	takeLimit uint64,
	bindVar int,
	prependVars []interface{},
	fieldNames []string,
) ([]interface{}, error) {
	var err error
	filters := make([]*Filter, 0)
	varReplacements := make([]interface{}, 0)
	take := r.FormValue("take")
	skip := r.FormValue("skip")
	filtersEncoded := r.FormValue("filters")
	sortEncoded := r.FormValue("sort")

	intTake, err := strconv.ParseUint(take, 10, 32)

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

	ApplyLimit(query)
	varReplacements = append(varReplacements, take, skip)
	newQuery := sqlx.Rebind(bindVar, *query)
	*query = newQuery

	// for i := range varReplacements {
	// 	if varReplacements[i] == nil {
	// 		varReplacements = append(varReplacements[:i], varReplacements[i+1:]...)
	// 	}
	// }

	return varReplacements, nil
}

func CountSelect(column string) string {
	return fmt.Sprintf("count(%s) as total", column)
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
					replacements = append(replacements, v.Value)
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
