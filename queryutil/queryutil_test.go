package queryutil

// func TestApplyGrouping(t *testing.T) {
// 	//var filterArray []Filter
// 	filter := `%5B%7B"field"%3A"email"%2C"operator"%3A"contains"%2C"value"%3A"t"%7D%2C%7B"field"%3A"phoneNumber"%2C"operator"%3A"startswith"%2C"value"%3A"5"%7D%5D`
// 	// boom, err := url.QueryUnescape(filter)

// 	// if err != nil {
// 	// 	t.Errorf("decode boom: %s", err)
// 	// }

// 	// err = json.Unmarshal([]byte(boom), &filterArray)

// 	// if err != nil {
// 	// 	fmt.Printf(boom)
// 	// 	t.Errorf("decode err: %s", err)
// 	// }

// 	query :=
// 		`
// 		select
// 			c.*,
// 			call_status.status as "call_status.status",
// 			state.name as "state.name",
// 			area.area_name as "area.area_name"
// 			from
// 				contractor c
// 			join
// 				call_status on c.call_status_id = call_status.id
// 			join
// 				area on c.area_id = area.id
// 			join
// 				state on c.state_id = state.id
// 		`

// 	err := ApplyFilter(&query, true, filter)

// 	if err != nil {
// 		t.Errorf("decode err: %s", err)
// 	}

// 	t.Errorf("query value err: %s", query)

// 	newString := sqlx.Rebind(sqlx.DOLLAR, query)

// 	t.Errorf("new string: %s", newString)

// }

type MockFormRequest struct{}

func (m *MockFormRequest) FormValue(key string) string {
	if key == "take" {
		return "20"
	} else if key == "skip" {
		return "0"
	} else if key == "filters" {
		return `%5B%7B"field"%3A"email"%2C"operator"%3A"contains"%2C"value"%3A"t"%7D%2C%7B"field"%3A"phoneNumber"%2C"operator"%3A"startswith"%2C"value"%3A"5"%7D%5D`
	} else {
		return `%7B"dir"%3A"asc"%2C"field"%3A"city"%7D`
	}
}

// func TestApplyAll(t *testing.T) {
// 	mockFormRequest := &MockFormRequest{}
// 	countQuery :=
// 		`
// 	select
// 		count(c.id) as count
// 		from
// 			contractor c
// 		join
// 			call_status on c.call_status_id = call_status.id
// 		join
// 			area on c.area_id = area.id
// 		join
// 			state on c.state_id = state.id
// 	`

// 	query :=
// 		`
// 		select
// 			c.*,
// 			call_status.status as "call_status.status",
// 			state.name as "state.name",
// 			area.area_name as "area.area_name"
// 			from
// 				contractor c
// 			join
// 				call_status on c.call_status_id = call_status.id
// 			join
// 				area on c.area_id = area.id
// 			join
// 				state on c.state_id = state.id
// 		`
// 	queryWithArea := query + " where contractor.area_id = ? and "
// 	countQueryWithArea := countQuery + " where contractor.area_id = ? and "

// 	replacements, err := ApplyAll(mockFormRequest, &query, true, true, sqlx.DOLLAR, nil)

// 	if err != nil {
// 		t.Errorf("apply filter err: %s", err)
// 	}

// 	t.Errorf("replacement1: %s\n", replacements)
// 	t.Errorf("query: %s\n", query)

// 	replacements2, err := ApplyAll(mockFormRequest, &queryWithArea, false, true, sqlx.DOLLAR, []interface{}{"1"})

// 	if err != nil {
// 		t.Errorf("apply filter err: %s", err)
// 	}

// 	t.Errorf("replacement2", replacements2)
// 	t.Errorf("queryWithArea: %s\n", queryWithArea)

// 	replacements, err = ApplyAll(mockFormRequest, &countQuery, true, false, sqlx.DOLLAR, nil)

// 	if err != nil {
// 		t.Errorf("apply filter err: %s", err)
// 	}

// 	t.Errorf("replacement1: %s\n", replacements)
// 	t.Errorf("count query: %s\n", countQuery)

// 	replacements2, err = ApplyAll(mockFormRequest, &countQueryWithArea, false, false, sqlx.DOLLAR, []interface{}{"1"})

// 	if err != nil {
// 		t.Errorf("apply filter err: %s", err)
// 	}

// 	t.Errorf("replacement2", replacements2)
// 	t.Errorf("count queryWithArea: %s\n", countQueryWithArea)
// }

// func TestParseWhereQuery(t *testing.T) {
// 	fromClause := " wherehouse"
// 	result := parseWhereQuery(fromClause, 0)

// 	if result != " where " {
// 		t.Errorf("should of got where: got %s", result)
// 	}
// }
