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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/TravisS25/httputil/apiutil"
)

const (
	TokenHeader     = "X-CSRF-TOKEN"
	CookieHeader    = "Cookie"
	SetCookieHeader = "Set-Cookie"

	ResponseErrorMessage = "apitesting: Result values: %v;\n expected results: %v\n"
)

var (
	True  = true
	False = false
)

type id interface {
	GetID() interface{}
}

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
	// ValidResponse func(bodyResponse io.Reader) (bool, error)
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

func (i intIDResponse) GetID() interface{} {
	return i.ID
}

type Int64ID struct {
	ID int64 `json:"id,string"`
}

type filteredIntIDResponse struct {
	Data  []intIDResponse `json:"data"`
	Count int             `json:"count"`
}

type filteredInt64IDResponse struct {
	Data  []Int64ID `json:"data"`
	Count int       `json:"count"`
}

type Int64MapID map[string]interface{}
type Int64MapSliceID map[string]interface{}

type Response struct {
	ExpectedResult       interface{}
	ValidateResponseFunc func(bodyResponse io.Reader, expectedResult interface{}) error
}

func RunTestCasesV2(t *testing.T, deferFunc func() error, testCases []TestCase) {
	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(v *testing.T) {
			panicked := true
			defer func() {
				if deferFunc != nil {
					if panicked {
						err := deferFunc()

						if err != nil {
							fmt.Printf(err.Error())
						}
					}
				}
			}()
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
					apiutil.CheckError(err, "")
				}
			}

			if testCase.ValidateResponse.ValidateResponseFunc != nil {
				err = testCase.ValidateResponse.ValidateResponseFunc(
					rr.Body,
					testCase.ValidateResponse.ExpectedResult,
				)

				if err != nil {
					v.Errorf(err.Error() + "\n")
				}
			}

			if testCase.PostResponseValidation != nil {
				if err = testCase.PostResponseValidation(); err != nil {
					v.Errorf(err.Error() + "\n")
				}
			}

			panicked = false
		})
	}
}

// RunTestCases takes the given list of TestCase structs and loops through
// and applies tests based on each TestCase struct config
//
// Deprecated for RunTestCasesV2
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
					apiutil.CheckError(err, "")
				}
			}

			if testCase.ValidateResponse.ValidateResponseFunc != nil {
				err = testCase.ValidateResponse.ValidateResponseFunc(
					rr.Body,
					testCase.ValidateResponse.ExpectedResult,
				)

				if err != nil {
					v.Errorf(err.Error() + "\n")
					apiutil.CheckError(err, "")
				}
			}

			if testCase.PostResponseValidation != nil {
				if err = testCase.PostResponseValidation(); err != nil {
					v.Errorf(err.Error() + "\n")
					apiutil.CheckError(err, "")
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
			return errors.New("apitesting: Expected result should be []int")
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

	case []Int64ID:
		expectedIDs, ok := expectedResult.([]int64)

		if !ok {
			return errors.New("apitesting: Expected result should be []int64")
		}

		convertedResults := result.([]Int64ID)
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
			return errors.New("apitesting: Expected result should be []int")
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
	case filteredInt64IDResponse:
		expectedIDs, ok := expectedResult.([]int64)

		if !ok {
			return errors.New("apitesting: Expected result should be []int64")
		}

		convertedResults := result.(filteredInt64IDResponse)
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
			return errors.New("apitesting: Expected result should be int")
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
	case Int64ID:
		expectedID, ok := expectedResult.(int64)

		if !ok {
			return errors.New("apitesting: Expected result should be int64")
		}

		convertedResult := result.(Int64ID)
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
	case Int64MapID:
		expectedMap, ok := expectedResult.(map[string]interface{})

		if !ok {
			return errors.New("apitesting: Expected result should be map[string]interface{}")
		}

		responseResults := result.(map[string]interface{})
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults) != len(expectedMap) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults,
				expectedMap,
			)
			return errors.New(errorMessage)
		}

		// Loop through given expected map of values and check whether the key
		// values are within the body response key values
		//
		// If key exists, determine through reflection if value is struct or
		// slice and compare ids to determine if expected map value
		// equals value of body response map
		//
		// If key does not exist, return err
		for k := range expectedMap {
			if responseVal, ok := responseResults[k]; ok {
				// Get json bytes from body response
				buf := bytes.Buffer{}
				buf.ReadFrom(bodyResponse)

				// Determine kind for interface{} value so we can
				// properly convert to typed json
				switch reflect.TypeOf(responseVal).Kind() {
				// If interface{} value is struct, then convert convertedResults
				// and expectedMap to typed json (Int64ID) to compare id
				case reflect.Struct:
					var expectedInt64ID Int64ID
					var responseInt64ID Int64ID

					expectedIDBytes, err := json.Marshal(expectedMap[k])

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					responseIDBytes, err := json.Marshal(responseVal)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(expectedIDBytes, &expectedInt64ID)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(responseIDBytes, &responseInt64ID)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					if expectedInt64ID.ID != responseInt64ID.ID {
						errorMessage := fmt.Sprintf(
							ResponseErrorMessage,
							responseResults,
							expectedMap,
						)
						return errors.New(errorMessage)
					}

				// If interface{} value is slice, then convert body response
				// and expectedMap to typed json (int64MapSliceID) to then
				// loop through and compare ids
				case reflect.Slice:
					var expectedInt64IDs []Int64ID
					var responseInt64IDs []Int64ID

					expectedIDsBytes, err := json.Marshal(expectedMap[k])

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					responseIDsBytes, err := json.Marshal(responseVal)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(expectedIDsBytes, &expectedInt64IDs)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(responseIDsBytes, &responseInt64IDs)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					for _, v := range expectedInt64IDs {
						containsID := false

						for _, t := range responseInt64IDs {
							if t.ID == v.ID {
								containsID = true
								break
							}
						}

						if !containsID {
							message := fmt.Sprintf(
								"apitesting: Slice response does not contain %d", v.ID,
							)
							return errors.New(message)
						}
					}

				// Id interface{} valie is neither struct or slice, then return err
				default:
					return errors.New("apitesting: not valid type")
				}
			} else {
				message := fmt.Sprintf("apitesting: key %s not in results from body", k)
				return errors.New(message)
			}
		}

	default:
		return errors.New("apitesting: Invalid result type passed")
	}

	return nil
}

func ValidateObjectID64Response(bodyResponse io.Reader, expectedResult interface{}) error {
	result := Int64ID{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateObjectIDResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	result := intIDResponse{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateFilteredIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	result := filteredIntIDResponse{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateFilteredInt64ArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	result := filteredInt64IDResponse{}
	return validateIDResponse(bodyResponse, result, expectedResult)
}

func ValidateIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	resultIDs := make([]intIDResponse, 0)
	return validateIDResponse(bodyResponse, resultIDs, expectedResult)
}

func ValidateInt64ArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	resultIDs := make([]Int64ID, 0)
	return validateIDResponse(bodyResponse, resultIDs, expectedResult)
}

func ValidateStringResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	response, err := ioutil.ReadAll(bodyResponse)

	if err != nil {
		return err
	}

	if result, ok := expectedResult.(string); ok {
		if string(response) != result {
			return fmt.Errorf("Response and expected strings did not match")
		}

		return nil
	}

	return fmt.Errorf("Expected result must be string")
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
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

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
