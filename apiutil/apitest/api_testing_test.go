package apitest

import (
	"bytes"
	"testing"
)

func ExampleRunTestCases() {

}

func TestNewFileUploadRequest(t *testing.T) {
	req, _ := NewFileUploadRequest(
		"/url",
		map[string]string{
			"boo": `{"Hello": "There"}`,
		},
		"document",
		"/home/travis/Programming/Go/src/bitbucket.org/TravisS25/go-pac/src/server/test-files/document.html",
	)

	foo := &bytes.Buffer{}
	foo.ReadFrom(req.Body)
	t.Errorf("body: %s\n", foo.String())
}
