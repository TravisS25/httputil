package apiutil

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

var (
	router = mux.NewRouter()
)

func init() {
	//router.HandleFunc("")
}

type Test struct {
	Bar string `json:"bar"`
	Foo string `json:"foo"`
}

func ExampleServerError() {
	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		// ... Doing something and catch unresolvable error
		err := errors.New("test error")

		if err != nil {
			// This will write back to client with a 500 status with custom message
			// and then log error at /var/test.log
			ServerError(w, err, "custom message")
		}
	})
}

// func TestJsonPayload(t *testing.T) {
// 	type MockJson struct {
// 		ID     int64  `json:"id,string"`
// 		Public string `json:"public"`
// 		Secret string `json:"secret"`
// 	}

// 	err := jsonPayload(
// 		map[string]interface{}{
// 			"mocks": []*MockJson{
// 				{
// 					ID:     1,
// 					Public: "public1",
// 					Secret: "Secret",
// 				},
// 				{
// 					ID:     2,
// 					Public: "public1",
// 					Secret: "Secret",
// 				},
// 				{
// 					ID:     3,
// 					Public: "public1",
// 					Secret: "Secret",
// 				},
// 			},
// 		},
// 		map[string]interface{}{
// 			"mocks": []interface{}{
// 				map[string]interface{}{
// 					"secret": true,
// 				},
// 			},
// 		},
// 		"",
// 	)

// 	if err != nil {
// 		t.Errorf(err.Error())
// 	}
// }

func ExampleHasBodyError() {
	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		// ... Doing something
		if HasBodyError(w, r) {

		}
	})
}
