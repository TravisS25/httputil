package queryutil

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/TravisS25/httputil"
)

type MockRower struct {
	getScan    func(dest ...interface{}) error
	getNext    func() bool
	getColumns func() ([]string, error)
}

func (m *MockRower) Scan(dest ...interface{}) error {
	return m.getScan(dest...)
}

func (m *MockRower) Next() bool {
	return m.getNext()
}

func (m *MockRower) Columns() ([]string, error) {
	return m.getColumns()
}

type MockQuerier struct {
	getQuery    func(query string, args ...interface{}) (httputil.Rower, error)
	getQueryRow func(query string, args ...interface{}) httputil.Scanner
}

func (m *MockQuerier) Query(query string, args ...interface{}) (httputil.Rower, error) {
	return m.getQuery(query, args...)
}

func (m *MockQuerier) QueryRow(query string, args ...interface{}) httputil.Scanner {
	return m.getQueryRow(query, args...)
}

type MockFormRequest struct{}

func (m *MockFormRequest) FormValue(key string) string {
	switch key {
	case "groups":
		return `[{"field": "foo.statusID"}]`
	case "filters":
		return `[{"field": "foo.number", "operator": "eq", "value":"test"}]`
	case "sorts":
		return `[{"field": "foo.dateExpired", "dir": "desc"}]`
	case "take":
		return `20`
	case "skip":
		return `0`
	case "invalid":
		return `invalid`
	default:
		return ""
	}
}

var (
	groupExp        = regexp.MustCompile(`(?i)(\n|\t|\s)group(\n|\t|\s)`)
	sortExp         = regexp.MustCompile(`(?i)(\n|\t|\s)order(\n|\t|\s)`)
	testMockRequest = &MockFormRequest{}
	testQuery       = `
	select 
		foo.*
	from
		foo
	where
		foo.name = 'test'
`
	testFields = map[string]FieldConfig{
		"foo.number": FieldConfig{
			DBField: "foo.number",
			OperationConf: OperationConfig{
				CanFilterBy: true,
				CanGroupBy:  true,
				CanSortBy:   true,
			},
		},
		"foo.dateExpired": FieldConfig{
			DBField: "foo.date_expired",
			OperationConf: OperationConfig{
				CanFilterBy: true,
				CanGroupBy:  true,
				CanSortBy:   true,
			},
		},
		"foo.statusID": FieldConfig{
			DBField: "foo.status_id",
			OperationConf: OperationConfig{
				CanFilterBy: true,
				CanGroupBy:  true,
				CanSortBy:   true,
			},
		},
	}
)

func TestDecodeFilters(t *testing.T) {
	var filters []Filter
	var err error

	if filters, err = DecodeFilters(testMockRequest, "filters"); err != nil {
		t.Fatalf(err.Error())
	}

	if len(filters) != 1 {
		t.Fatalf("Should have one replacement variable\n")
	}
}

func TestReplaceFilterFields(t *testing.T) {
	f := []Filter{
		{
			Field:    "foo.number",
			Operator: "eq",
			Value:    "test",
		},
	}
	r, err := ReplaceFilterFields(
		&testQuery, f, testFields,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if len(r) != 1 {
		t.Fatalf("Should have replacement variable of 1; got %d\n", len(r))
	}
}

func TestGetFilterReplacements(t *testing.T) {
	var r []interface{}
	var err error
	q := testQuery

	if _, r, err = GetFilterReplacements(
		testMockRequest,
		&q,
		"filters",
		nil,
		testFields,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if len(r) != 1 {
		t.Fatalf("Should have one replacement variable: got %d\n", len(r))
	}
}

func TestDecodeGroups(t *testing.T) {
	var groups []Group
	var err error

	if groups, err = DecodeGroups(testMockRequest, "groups"); err != nil {
		t.Fatalf(err.Error())
	}

	if len(groups) != 1 {
		t.Fatalf("Should have one replacement variable\n")
	}
}

func TestGetGroupReplacements(t *testing.T) {
	//var r []interface{}
	var err error
	q := testQuery
	f := testQuery
	f +=
		`
	group by
		foo.bar
	`

	if _, err = GetGroupReplacements(testMockRequest, &q, "groups", nil, testFields); err != nil {
		t.Fatalf(err.Error())
	}

	if val := strings.Contains(q, "group by"); !val {
		t.Fatalf("Query should contain 'group by' clause\n  query: %s", q)
	}

	if _, err = GetGroupReplacements(testMockRequest, &f, "groups", nil, testFields); err != nil {
		t.Fatalf(err.Error())
	}

	allStrings := groupExp.FindAllString(f, -1)

	if len(allStrings) != 1 {
		t.Fatalf("Should not have added 'group by' to query\n  new query: %s\n", f)
	}
}

func TestDecodeSorts(t *testing.T) {
	var sorts []Sort
	var err error

	if sorts, err = DecodeSorts(testMockRequest, "sorts"); err != nil {
		t.Fatalf(err.Error())
	}

	if len(sorts) != 1 {
		t.Fatalf("Should have one replacement variable\n")
	}

	if sorts[0].Field != "foo.number" || sorts[0].Dir != "desc" {
		t.Fatalf("sort not properly decoded\n")
	}
}

func TestGetSortReplacements(t *testing.T) {
	//var r []interface{}
	var err error
	q := testQuery
	f := testQuery
	f +=
		`
	sort by
		foo.bar desc
	`

	if _, err = GetSortReplacements(testMockRequest, &q, "sorts", nil, testFields); err != nil {
		t.Fatalf(err.Error())
	}

	if val := strings.Contains(q, "order by"); !val {
		t.Fatalf("Query should contain 'order by' clause\n  query: %s", q)
	}

	if _, err = GetSortReplacements(testMockRequest, &f, "sorts", nil, testFields); err != nil {
		t.Fatalf(err.Error())
	}

	allStrings := sortExp.FindAllString(f, -1)

	if len(allStrings) != 1 {
		t.Fatalf("Should not have added 'order by' to query\n  new query: %s\n", f)
	}
}

func TestGetQueriedResults(t *testing.T) {
	var err error

	db := &MockQuerier{
		getQuery: func(q string, args ...interface{}) (httputil.Rower, error) {
			return &MockRower{}, nil
		},
	}
	q :=
		`
	select
		min(foo.id) as id,
	from
		foo
	`

	s := sqlx.DOLLAR

	if _, err = GetPreQueryResults(
		&q,
		nil,
		testFields,
		testMockRequest,
		db,
		ParamConfig{},
		QueryConfig{
			SQLBindVar: &s,
			PrependGroupFields: []Group{
				{
					Field: "foo.number",
				},
			},
			// PrependSortFields: []Sort{
			// 	{
			// 		Field: "foo.dateExpired",
			// 		Dir:   "asc",
			// 	},
			// },
		},
	); err != nil {
		t.Fatalf(err.Error())
	}

	t.Fatalf("query: %s\n", q)
}

func TestGetLimitWithOffsetReplacements(t *testing.T) {
	var err error

	q :=
		`
	select
		foo.*
	from
		foo.*
	where
		foo.id = $1
	`
	_, err = GetLimitWithOffsetReplacements(
		testMockRequest,
		&q,
		"take",
		"skip",
		10,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	_, err = GetLimitWithOffsetReplacements(
		testMockRequest,
		&q,
		"invalid",
		"skip",
		10,
	)

	if err == nil {
		t.Fatalf("Should be error")
	}
	err = nil
}
