package formutil

var (
	formValdiation      Form
	formCacheValidation FormCache
)

type TestFormCacheValidation struct {
	Foo string `json:"foo"`
	Boo string `json:"boo"`

	FormValidation
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
