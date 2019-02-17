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

// func ExampleHasFormErrors() {
// 	var Test struct {
// 		Bar string `json:"bar"`
// 		Foo string `json:"foo"`
// 	}

// 	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
// 		// ... Doing something
// 		var test Test

// 		if err != nil {
// 			ServerError(w, err, "custom message", "/var/test.log")
// 		}

// 		return
// 	})
// }

func ExampleHasBodyError() {
	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		// ... Doing something
		if HasBodyError(w, r) {

		}
	})
}
