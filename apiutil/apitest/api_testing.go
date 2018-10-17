package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/TravisS25/httputil/apiutil"
)

const (
	TokenHeader     = "X-CSRF-TOKEN"
	CookieHeader    = "Cookie"
	SetCookieHeader = "Set-Cookie"

	ResponseErrorMessage = "Result values: %v;\n expected results: %v\n"
)

// TestCase is config struct used in conjunction with
// the RunTestCases function
type TestCase struct {
	// TestName is name of given test
	TestName string
	// Method is http method used for request eg. "get", "post", "put", "delete"
	Method string
	// RequestURL is the url you want to test
	RequestURL string
	// ExpectedStatus is http response code you expect to retrieve from request
	ExpectedStatus int
	// ExpectedBody is the expected response, if any, that given response will have
	ExpectedBody string
	// ContextValues is used for adding context values with request
	ContextValues map[interface{}]interface{}
	// Form is information that you wish to body of request
	// This is generally used for post/put requests
	Form interface{}
	// Handler is the request handler that you which to test
	Handler http.Handler
	// ValidResponse allows user to take in response from api end
	// and determine if the given response is the expected one
	//ValidResponse func(bodyResponse io.Reader) (bool, error)
	ValidateResponse Response
	// PostResponse is used to validate anything a user wishes after api is
	// done executing.  This is mainly intended to be used for querying
	// against a database after POST/PUT/DELETE request to validate that
	// proper things were written to the database.  Could also be used
	// for clean up
	PostResponseValidation func() error
}

type intIDResponse struct {
	ID int `json:"id"`
}

type filteredIntIDResponse struct {
	Data  []intIDResponse `json:"data"`
	Count int             `json:"count"`
}

type Response struct {
	ExpectedResult       interface{}
	ValidateResponseFunc func(bodyResponse io.Reader, expectedResult interface{}) error
}

// RunTestCases takes the given list of TestCase structs and loops through
// and applies tests based on each TestCase struct config
func RunTestCases(t *testing.T, testCases []TestCase) {
	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(v *testing.T) {
			var req *http.Request
			var err error

			// If Form option is nil, init req without added parameters
			// Else json encode given form and apply to request
			if testCase.Form == nil {
				req, err = http.NewRequest(testCase.Method, testCase.RequestURL, nil)
			} else {
				var buffer bytes.Buffer
				encoder := json.NewEncoder(&buffer)
				encoder.Encode(&testCase.Form)
				req, err = http.NewRequest(testCase.Method, testCase.RequestURL, &buffer)
			}

			if err != nil {
				v.Fatal(err)
			}

			// If ContextValues is not nil, apply given context values to req
			if testCase.ContextValues != nil {
				ctx := req.Context()

				for key, value := range testCase.ContextValues {
					ctx = context.WithValue(ctx, key, value)
				}

				req = req.WithContext(ctx)
			}

			// Init recorder that will be written to based on the status
			// we get from created request
			rr := httptest.NewRecorder()
			testCase.Handler.ServeHTTP(rr, req)

			// If status is not what was expected, print error
			if status := rr.Code; status != testCase.ExpectedStatus {
				v.Errorf("got status %d; want %d\n", status, testCase.ExpectedStatus)
				v.Errorf("body response: %s\n", rr.Body.String())
			}

			// If ExpectedBody option was given and does not equal what was
			// returned, print error
			if testCase.ExpectedBody != "" {
				if testCase.ExpectedBody != rr.Body.String() {
					v.Errorf("got body %s; want %s\n", rr.Body.String(), testCase.ExpectedBody)
				}
			}

			if testCase.ValidateResponse.ValidateResponseFunc != nil {
				err = testCase.ValidateResponse.ValidateResponseFunc(
					rr.Body,
					testCase.ValidateResponse.ExpectedResult,
				)

				if err != nil {
					v.Errorf(err.Error())
				}
			}

			if testCase.PostResponseValidation != nil {
				if err = testCase.PostResponseValidation(); err != nil {
					v.Errorf(err.Error())
				}
			}
		})
	}
}

func validateIDResponse(bodyResponse io.Reader, result interface{}, expectedResult interface{}) error {
	foundResult := false

	switch result.(type) {
	case []intIDResponse:
		expectedIDs, ok := expectedResult.([]int)

		if !ok {
			return errors.New("Expected result should be []int")
		}

		convertedResults := result.([]intIDResponse)
		err := SetJSONFromResponse(bodyResponse, &convertedResults)

		if err != nil {
			return err
		}

		if len(convertedResults) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				convertedResults,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range convertedResults {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					convertedResults,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break

	case filteredIntIDResponse:
		expectedIDs, ok := expectedResult.([]int)

		if !ok {
			return errors.New("Expected result should be []int")
		}

		convertedResults := result.(filteredIntIDResponse)
		err := SetJSONFromResponse(bodyResponse, &convertedResults)

		if err != nil {
			return err
		}

		if len(convertedResults.Data) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				convertedResults.Data,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range convertedResults.Data {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					convertedResults.Data,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break
	case intIDResponse:
		expectedID, ok := expectedResult.(int)

		if !ok {
			return errors.New("Expected result should be int")
		}

		convertedResult := result.(intIDResponse)
		err := SetJSONFromResponse(bodyResponse, &convertedResult)

		if err != nil {
			return err
		}

		if convertedResult.ID != expectedID {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				convertedResult.ID,
				expectedID,
			)
			return errors.New(errorMessage)
		}
	default:
		return errors.New("Invalid result type passed")
	}

	return nil
}

func ValidateObjectIDResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	result := intIDResponse{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateFilteredIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	result := filteredIntIDResponse{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	resultIDs := make([]intIDResponse, 0)
	return validateIDResponse(bodyResponse, resultIDs, expectedResult)
}

// SetJSONFromResponse takes io.Reader which will generally be a response from
// api endpoint and applies the json representation to the passed interface
func SetJSONFromResponse(bodyResponse io.Reader, item interface{}) error {
	response, err := ioutil.ReadAll(bodyResponse)

	if err != nil {
		return err
	}

	err = json.Unmarshal(response, &item)

	if err != nil {
		return err
	}

	return nil
}

// ResponseError is a wrapper function for a http#Response and handling errors
func ResponseError(t *testing.T, res *http.Response, expectedStatus int, err error) {
	if err != nil {
		t.Fatalf("err on response: %s", err.Error())
	} else {
		if res.StatusCode != expectedStatus {
			t.Errorf("Got %d error, should be %d\n", res.StatusCode, expectedStatus)

			if res.Body != nil {
				resErr, _ := ioutil.ReadAll(res.Body)
				t.Errorf("Body response: %s\n", string(resErr))
			}
		}
	}
}

// LoginUser takes email and password along with login url and form information
// to use to make a POST request to login url and if successful, return user cookie
func LoginUser(email, password, loginURL string, loginForm interface{}, ts *httptest.Server) (string, error) {
	client := &http.Client{}

	baseURL := ts.URL
	fullURL := baseURL + loginURL

	req, err := http.NewRequest("GET", fullURL, nil)

	if err != nil {
		return "", err
	}

	res, err := client.Do(req)

	if err != nil {
		return "", err
	}

	token := res.Header.Get(TokenHeader)
	csrf := res.Header.Get(SetCookieHeader)
	buffer := apiutil.GetJSONBuffer(loginForm)
	req, err = http.NewRequest("POST", fullURL, &buffer)

	if err != nil {
		return "", err
	}

	req.Header.Set(TokenHeader, token)
	req.Header.Set(CookieHeader, csrf)
	res, err = client.Do(req)

	if err != nil {
		return "", err
	}

	return res.Header.Get(SetCookieHeader), nil
}

// NewFileUploadRequest creates a new file upload http request with optional extra params
func NewFileUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	file.Close()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, fi.Name())
	if err != nil {
		return nil, err
	}
	part.Write(fileContents)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}
