package httputil

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// CheckError simply prints given error in verbose to stdout
func CheckError(err error, customMessage string) {
	err = errors.Wrap(err, customMessage)
	fmt.Printf("%+v\n", err)
}

func InsertAt(slice []interface{}, val interface{}, idx int) []interface{} {
	if len(slice) == 0 {
		slice = append(slice, val)
	} else {
		slice = append(slice, 0)
		copy(slice[idx+1:], slice[idx:])
		slice[idx] = val
	}

	return slice
}

// PathRegex is a work around for the fact that injecting and retrieving a route into
// mux is quite complex without spinning up an entire server
type PathRegex func(r *http.Request) (string, error)
