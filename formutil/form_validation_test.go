package formutil

import (
	"bytes"
	"encoding/json"
	"testing"
)

type TestFormCacheValidation struct {
	Foo string `json:"foo"`
	Boo string `json:"boo"`

	FormValidation
}

func TestInt64(t *testing.T) {
	//r, _ := http.NewRequest(http.MethodGet, "/url", nil)
	type test struct {
		Foo Int64 `json:"foo"`
	}

	var v test

	b := &bytes.Buffer{}
	b.WriteString(`{"foo": null}`)

	dec := json.NewDecoder(b)
	err := dec.Decode(&v)

	if err != nil {
		t.Errorf("error: %s", err.Error())
	}

	t.Errorf("foo value %v", v.Foo)
}

func TestValidateDateRule(t *testing.T) {
	v := validateDateRule{}
	v.Validate(nil)
	t.Errorf("boom")
}

// func (t TestFormCacheValidation) Validate(item interface{}) error {
// 	form := item.(TestFormCacheValidation)

// }

// func init() {

// }

// func initFormCacheValidation() *FormValidation {

// }

// func TestUnmarshalIntPtr(t *testing.T) {
// 	type Boom struct {
// 		Test UnmarshalIntPtr `json:"test"`
// 	}

// 	id := 1
// 	boom := Boom{Test: UnmarshalIntPtr{value: &id}}

// 	var dest Boom
// 	reqBodyBytes := new(bytes.Buffer)
// 	json.NewEncoder(reqBodyBytes).Encode(boom)

// 	err := json.Unmarshal(reqBodyBytes.Bytes(), &dest)

// 	if err != nil {
// 		t.Errorf("Error: %s", err)
// 	}
// }

func ExampleFormCache() {

}
