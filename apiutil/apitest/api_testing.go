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

	ResponseErrorMessage    = "apitesting: Result values: %v;\n expected results: %v\n"
	MapResponseErrorMessage = "apitesting: Key value \"%s\":\n Result values: %v;\n expected results: %v\n"
)

const (
	intMapIDResult = iota + 1
	int64MapIDResult
	intArrayIDResult
	int64ArrayIDResult
	intFilteredIDResult
	int64FilteredIDResult
	intObjectIDResult
	int64ObjectIDResult
)

var (
	True  = true
	False = false
)

type id interface {
	GetID() interface{}
}

type MockHandler struct {
	ServeHTTPFunc func(http.ResponseWriter, *http.Request)
}

func (m *MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.ServeHTTPFunc(w, r)
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

type intID struct {
	ID int `json:"id"`
}

type int64ID struct {
	ID int64 `json:"id,string"`
}

type filteredIntID struct {
	Data  []intID `json:"data"`
	Count int     `json:"count"`
}

type filteredInt64ID struct {
	Data  []int64ID `json:"data"`
	Count int       `json:"count"`
}

type Response struct {
	ExpectedResult       interface{}
	ValidateResponseFunc func(bodyResponse io.Reader, expectedResult interface{}) error
}

func NewRequestWithForm(method, url string, form interface{}) (*http.Request, error) {
	if form != nil {
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		encoder.Encode(&form)
		return http.NewRequest(method, url, &buffer)
	}

	fmt.Printf("past form nil\n")
	return http.NewRequest(method, url, nil)
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

func validateIDResponse(bodyResponse io.Reader, result int, expectedResults interface{}) error {
	foundResult := false

	switch result {
	case intArrayIDResult:
		expectedIDs, ok := expectedResults.([]int)

		if !ok {
			return errors.New("apitesting: Expected result should be []int")
		}

		var responseResults []intID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range responseResults {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					responseResults,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break

	case int64ArrayIDResult:
		expectedIDs, ok := expectedResults.([]int64)

		if !ok {
			return errors.New("apitesting: Expected result should be []int64")
		}

		var responseResults []int64ID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range responseResults {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					responseResults,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break

	case intFilteredIDResult:
		expectedIDs, ok := expectedResults.([]int)

		if !ok {
			return errors.New("apitesting: Expected result should be []int")
		}

		var responseResults filteredIntID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults.Data) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults.Data,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range responseResults.Data {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					responseResults.Data,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break
	case int64FilteredIDResult:
		expectedIDs, ok := expectedResults.([]int64)

		if !ok {
			return errors.New("apitesting: Expected result should be []int64")
		}

		var responseResults filteredInt64ID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults.Data) != len(expectedIDs) {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults.Data,
				expectedIDs,
			)
			return errors.New(errorMessage)
		}

		for _, m := range expectedIDs {
			for _, v := range responseResults.Data {
				if m == v.ID {
					foundResult = true
					break
				}
			}

			if foundResult == false {
				errorMessage := fmt.Sprintf(
					ResponseErrorMessage,
					responseResults.Data,
					expectedIDs,
				)
				return errors.New(errorMessage)
			}

			foundResult = false
		}
		break
	case intObjectIDResult:
		expectedID, ok := expectedResults.(int)

		if !ok {
			return errors.New("apitesting: Expected result should be int")
		}

		var responseResults intID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if responseResults.ID != expectedID {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults.ID,
				expectedID,
			)
			return errors.New(errorMessage)
		}
	case int64ObjectIDResult:
		expectedID, ok := expectedResults.(int64)

		if !ok {
			return errors.New("apitesting: Expected result should be int64")
		}

		var responseResults int64ID
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if responseResults.ID != expectedID {
			errorMessage := fmt.Sprintf(
				ResponseErrorMessage,
				responseResults.ID,
				expectedID,
			)
			return errors.New(errorMessage)
		}
	case intMapIDResult:
		expectedMap, ok := expectedResults.(map[string]interface{})

		if !ok {
			return errors.New("apitesting: Expected result should be map[string]interface{}")
		}

		var responseResults map[string]interface{}
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
					var expectedIntID intID
					var responseIntID intID

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

					err = json.Unmarshal(expectedIDBytes, &expectedIntID)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(responseIDBytes, &responseIntID)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					if expectedIntID.ID != responseIntID.ID {
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
					var expectedIntIDs []intID
					var responseIntIDs []intID

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

					err = json.Unmarshal(expectedIDsBytes, &expectedIntIDs)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					err = json.Unmarshal(responseIDsBytes, &responseIntIDs)

					if err != nil {
						message := fmt.Sprintf("apitesting: %s", err.Error())
						return errors.New(message)
					}

					for _, v := range expectedIntIDs {
						containsID := false

						for _, t := range responseIntIDs {
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
	case int64MapIDResult:
		expectedMap, ok := expectedResults.(map[string]interface{})

		if !ok {
			return errors.New("apitesting: Expected result should be map[string]interface{}")
		}

		var responseResults map[string]interface{}
		err := SetJSONFromResponse(bodyResponse, &responseResults)

		if err != nil {
			return err
		}

		if len(responseResults) != len(expectedMap) {
			errorMessage := fmt.Sprintf(
				"Map result length %d; expected map length: %d",
				len(responseResults),
				len(expectedMap),
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
				// Determine kind for interface{} value so we can
				// properly convert to typed json
				if responseVal != nil {
					// Determine kind for interface{} value so we can
					// properly convert to typed json
					switch reflect.TypeOf(responseVal).Kind() {
					// If interface{} value is map, then convert convertedResults
					// and expectedMap to typed json (int64ID) to compare id
					case reflect.Map:
						var responseInt64ID int64ID

						expectedInt64, ok := expectedMap[k].(int64)

						if !ok {
							message := fmt.Sprintf(
								`apitesting: key value "%s" for "ExpectedResult" should be int64`,
								k,
							)
							return errors.New(message)
						}

						responseIDBytes, err := json.Marshal(responseVal)

						if err != nil {
							message := fmt.Sprintf("apitesting: %s", err.Error())
							return errors.New(message)
						}

						err = json.Unmarshal(responseIDBytes, &responseInt64ID)

						if err != nil {
							message := fmt.Sprintf("apitesting: %s", err.Error())
							return errors.New(message)
						}

						if expectedInt64 != responseInt64ID.ID {
							errorMessage := fmt.Sprintf(
								MapResponseErrorMessage,
								k,
								responseInt64ID.ID,
								expectedInt64,
							)
							return errors.New(errorMessage)
						}

					// If interface{} value is slice, then convert body response
					// and expectedMap to typed json (int64MapSliceID) to then
					// loop through and compare ids
					case reflect.Slice:
						var responseInt64IDs []int64ID

						expectedInt64Slice, ok := expectedMap[k].([]int64)

						if !ok {
							message := fmt.Sprintf(`apitesting: key value "%s" for "ExpectedResult" should be []int64`, k)
							return errors.New(message)
						}

						responseIDsBytes, err := json.Marshal(responseVal)

						if err != nil {
							message := fmt.Sprintf("apitesting: %s", err.Error())
							return errors.New(message)
						}

						err = json.Unmarshal(responseIDsBytes, &responseInt64IDs)

						if err != nil {
							message := fmt.Sprintf("apitesting: %s", err.Error())
							return errors.New(message)
						}

						for _, v := range expectedInt64Slice {
							containsID := false

							for _, t := range responseInt64IDs {
								if t.ID == v {
									containsID = true
									break
								}
							}

							if !containsID {
								message := fmt.Sprintf(
									"apitesting: Slice response does not contain %d", v,
								)
								return errors.New(message)
							}
						}

					// Id interface{} valie is neither struct or slice, then return err
					default:
						return errors.New("apitesting: not valid type")
					}
				} else {
					if expectedMap[k] != nil {
						errorMessage := fmt.Sprintf(
							MapResponseErrorMessage,
							k,
							nil,
							expectedMap[k],
						)
						return errors.New(errorMessage)
					}
				}
			} else {
				message := fmt.Sprintf(`apitesting: key value "%s" not in results from body`, k)
				return errors.New(message)
			}
		}

	default:
		return errors.New("apitesting: Invalid result type passed")
	}

	return nil
}

// func validateMapID(result int, bodyResponse io.Reader, expectedResults interface{}) error {
// 	expectedMap, ok := expectedResults.(map[string]interface{})

// 	if !ok {
// 		return errors.New("apitesting: Expected result should be map[string]interface{}")
// 	}

// 	var responseResults map[string]interface{}
// 	err := SetJSONFromResponse(bodyResponse, &responseResults)

// 	if err != nil {
// 		return err
// 	}

// 	if len(responseResults) != len(expectedMap) {
// 		errorMessage := fmt.Sprintf(
// 			ResponseErrorMessage,
// 			responseResults,
// 			expectedMap,
// 		)
// 		return errors.New(errorMessage)
// 	}

// 	// Loop through given expected map of values and check whether the key
// 	// values are within the body response key values
// 	//
// 	// If key exists, determine through reflection if value is struct or
// 	// slice and compare ids to determine if expected map value
// 	// equals value of body response map
// 	//
// 	// If key does not exist, return err
// 	for k := range expectedMap {
// 		if responseVal, ok := responseResults[k]; ok {
// 			// Get json bytes from body response
// 			buf := bytes.Buffer{}
// 			buf.ReadFrom(bodyResponse)

// 			// Determine kind for interface{} value so we can
// 			// properly convert to typed json
// 			switch reflect.TypeOf(responseVal).Kind() {
// 			// If interface{} value is struct, then convert convertedResults
// 			// and expectedMap to typed json (Int64ID) to compare id
// 			case reflect.Struct:
// 				switch result {
// 				case intMapIDResult:

// 				default:

// 				}

// 				expectedIDBytes, err := json.Marshal(expectedMap[k])

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				responseIDBytes, err := json.Marshal(responseVal)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				err = json.Unmarshal(expectedIDBytes, &expectedIntID)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				err = json.Unmarshal(responseIDBytes, &responseIntID)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				if expectedIntID.ID != responseIntID.ID {
// 					errorMessage := fmt.Sprintf(
// 						ResponseErrorMessage,
// 						responseResults,
// 						expectedMap,
// 					)
// 					return errors.New(errorMessage)
// 				}

// 			// If interface{} value is slice, then convert body response
// 			// and expectedMap to typed json (int64MapSliceID) to then
// 			// loop through and compare ids
// 			case reflect.Slice:
// 				var expectedIntIDs []intID
// 				var responseIntIDs []intID

// 				expectedIDsBytes, err := json.Marshal(expectedMap[k])

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				responseIDsBytes, err := json.Marshal(responseVal)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				err = json.Unmarshal(expectedIDsBytes, &expectedIntIDs)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				err = json.Unmarshal(responseIDsBytes, &responseIntIDs)

// 				if err != nil {
// 					message := fmt.Sprintf("apitesting: %s", err.Error())
// 					return errors.New(message)
// 				}

// 				for _, v := range expectedIntIDs {
// 					containsID := false

// 					for _, t := range responseIntIDs {
// 						if t.ID == v.ID {
// 							containsID = true
// 							break
// 						}
// 					}

// 					if !containsID {
// 						message := fmt.Sprintf(
// 							"apitesting: Slice response does not contain %d", v.ID,
// 						)
// 						return errors.New(message)
// 					}
// 				}

// 			// Id interface{} valie is neither struct or slice, then return err
// 			default:
// 				return errors.New("apitesting: not valid type")
// 			}
// 		} else {
// 			message := fmt.Sprintf("apitesting: key %s not in results from body", k)
// 			return errors.New(message)
// 		}
// 	}
// }

func ValidateFilteredIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, intFilteredIDResult, expectedResult)
}

func ValidateFilteredInt64ArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, int64FilteredIDResult, expectedResult)
}

func ValidateIntArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, intArrayIDResult, expectedResult)
}

func ValidateInt64ArrayResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, int64ArrayIDResult, expectedResult)
}

func ValidateIntMapResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, intMapIDResult, expectedResult)
}

func ValidateInt64MapResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, int64MapIDResult, expectedResult)
}

func ValidateIntObjectResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, intObjectIDResult, expectedResult)
}

func ValidateInt64ObjectResponse(bodyResponse io.Reader, expectedResult interface{}) error {
	return validateIDResponse(bodyResponse, int64ObjectIDResult, expectedResult)
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
func LoginUser(url string, loginForm interface{}) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return "", err
	}

	res, err := client.Do(req)

	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		buf := bytes.Buffer{}
		buf.ReadFrom(res.Body)
		errorMessage := fmt.Sprintf("status code: %d\n  response: %s\n", res.StatusCode, buf.String())
		return "", errors.New(errorMessage)
	}

	token := res.Header.Get(TokenHeader)
	csrf := res.Header.Get(SetCookieHeader)
	buffer := apiutil.GetJSONBuffer(loginForm)
	req, err = http.NewRequest(http.MethodPost, url, &buffer)

	if err != nil {
		return "", err
	}

	req.Header.Set(TokenHeader, token)
	req.Header.Set(CookieHeader, csrf)
	res, err = client.Do(req)

	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		buf := bytes.Buffer{}
		buf.ReadFrom(res.Body)
		errorMessage := fmt.Sprintf("status code: %d\n  response: %s\n", res.StatusCode, buf.String())
		return "", errors.New(errorMessage)
	}

	// fmt.Printf("cookies: %v\n\n", res.Cookies())
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

func CheckResponse(method, url string, expectedStatus int, header http.Header, form interface{}) (*http.Response, error) {
	client := &http.Client{}
	buffer := &bytes.Buffer{}
	req, _ := NewRequestWithForm(method, url, form)
	req.Header = header
	res, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	if res.StatusCode != expectedStatus {
		buffer.ReadFrom(res.Body)
		message := fmt.Sprintf("got status %d; want %d\nresponse: %s", res.StatusCode, http.StatusOK, buffer.String())
		return nil, errors.New(message)
	}

	return res, nil
}
