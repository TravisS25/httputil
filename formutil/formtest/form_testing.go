package formtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/TravisS25/httputil/formutil"

	validation "github.com/go-ozzo/ozzo-validation"
)

type FormRequestConfig struct {
	// TestName is the name of current test - Required
	TestName string

	// Method is http method to use for request - Required
	Method string

	// URL is uri you wish to request - Required
	URL string

	// Validate is interface for struct that will validate form - required
	Validator formutil.RequestValidator

	// Form is form values to use to inject into request - Required
	Form interface{}

	// Instance is instance of a model in which a form might need, usually
	// on an edit request - Optional
	Instance interface{}

	// ContextValues are context#Context to use for request - Optional
	ContextValues map[interface{}]interface{}

	// PostExecute can be used to exec some logic that you may need to run inbetween test cases
	// such as clean up logic before the next test is run - Optional
	PostExecute func()

	// ValidationErrors is a map of what errors you expect to return from test
	// The key is the json name of the field and value is the error message the
	// field should return - Required
	ValidationErrors map[string]string

	InternalError string
}

// FormTestCase is config struct for function RunFormTests
type FormTestCase struct {
	// TestName is the name of current test - Required
	TestName string

	// ValidationErrors is a map of what errors you expect to return from test
	// The key is the json name of the field and value is the error message the
	// field should return - Required
	ValidationErrors map[string]string

	// Validator is interface that is responsible for validating "Form" variable - Required
	FormValidator formutil.Validator

	// // FormRequestValidator is updated interface that is responsible for validating "Form" variable
	// FormRequestValidator formutil.RequestValidator

	// Value that will be validated against.  Whatever struct that implements
	// Validator should properly cast this to whatever form you are validating - Required
	Form interface{}

	// // Request is standard http request used in conjunction with FormValidatorV2
	// Request *http.Request

	// IsValidForm tells RunFormTests whether or not you expect the form to
	// have any errors returned or not - Optional
	IsValidForm bool

	// // NumOfErrors determines how many errors a user expects the form to return
	// // Should be the same number of entries in the "ValidationErrors" map variable but
	// // adding NumOfErrors is a confirmation
	// // If NumOfErrors does not match count of actual errors, test will fail - Optional
	// NumOfErrors int

	// PostExecute can be used to exec some logic that you may need to run inbetween test cases
	// such as clean up logic before the next test is run - Optional
	PostExecute func()

	// InternalError is expected internal
	InternalError string
}

func RunFormTests(t *testing.T, formTests []FormTestCase) {
	for _, formTest := range formTests {
		t.Run(formTest.TestName, func(t *testing.T) {
			validateFormTests(t, formTest)
		})
	}
}

func RunRequestFormTests(t *testing.T, deferFunc func(), formTests []FormRequestConfig) {
	for _, formTest := range formTests {
		if formTest.TestName == "" {
			t.Fatalf("TestName required")
		}
		if formTest.Method == "" {
			t.Fatalf("Method required")
		}
		if formTest.URL == "" {
			t.Fatalf("URL required")
		}
		if formTest.Validator == nil {
			t.Fatalf("Validator required")
		}
		if formTest.Form == nil {
			t.Fatalf("Form required")
		}
		t.Run(formTest.TestName, func(t *testing.T) {
			panicked := true
			defer func() {
				if deferFunc != nil {
					if panicked {
						deferFunc()
					}
				}
			}()

			jsonBytes, err := json.Marshal(&formTest.Form)

			if err != nil {
				t.Fatalf(err.Error())
			}

			buf := bytes.NewBuffer(jsonBytes)
			req, err := http.NewRequest(formTest.Method, formTest.URL, buf)

			if err != nil {
				t.Fatalf(err.Error())
			}

			if formTest.ContextValues != nil {
				ctx := req.Context()

				for key, value := range formTest.ContextValues {
					ctx = context.WithValue(ctx, key, value)
				}

				req = req.WithContext(ctx)
			}

			_, err = formTest.Validator.Validate(req, formTest.Instance)

			if err == nil {
				if formTest.ValidationErrors != nil {
					t.Errorf("Form has no errors, but 'ValidationErrors' was passed\n")
				}
			} else {
				if validationErrors, ok := err.(validation.Errors); ok {
					containsErr := false
					for key, expectedVal := range formTest.ValidationErrors {
						if val, valid := validationErrors[key]; valid {
							if val.Error() != expectedVal {
								containsErr = true
								t.Errorf("Key \"%s\" threw err: \"%s\" \n expected: \"%s\" \n", key, val.Error(), expectedVal)
							}
						}
					}

					if containsErr {
						fmt.Println("------------------")
					}

					if len(formTest.ValidationErrors) != len(validationErrors) {
						if len(formTest.ValidationErrors) > len(validationErrors) {
							for k := range formTest.ValidationErrors {
								if _, ok := validationErrors[k]; !ok {
									t.Errorf("Key \"%s\" found in \"ValidationErrors\" that is not in form errors", k)
								}
							}
						} else {
							for k, v := range validationErrors {
								if _, ok := formTest.ValidationErrors[k]; !ok {
									t.Errorf("Key \"%s\" found in form errors that is not in \"ValidationErrors\"\n  Threw err: %s", k, v.Error())
								}
							}
						}
					}
				} else {
					if formTest.InternalError != err.Error() {
						t.Errorf("Internal Error: %s\n", err.Error())
					}
				}
			}

			panicked = false
		})
	}
}

func RunFormTestsV2(t *testing.T, deferFunc func(testName string), formTests []FormTestCase) {
	for _, formTest := range formTests {
		t.Run(formTest.TestName, func(t *testing.T) {
			panicked := true
			defer func() {
				if deferFunc != nil {
					if panicked {
						deferFunc(formTest.TestName)
					}
				}
			}()

			validateFormTests(t, formTest)
			panicked = false
		})
	}
}

// func GetFormRequests(bodyRequests map[string]FormRequestConfig) (map[string]*http.Request, error) {
// 	requests := make(map[string]*http.Request, 0)

// 	for k, v := range bodyRequests {
// 		if v.MultiPart != nil {

// 		} else {
// 			jsonBytes, err := json.Marshal(&v.Form)

// 			if err != nil {
// 				return nil, err
// 			}

// 			buf := bytes.NewBuffer(jsonBytes)
// 			req, err := http.NewRequest(v.Method, v.URL, buf)

// 			if err != nil {
// 				return nil, err
// 			}

// 			if v.ContextValues != nil {
// 				ctx := req.Context()

// 				for key, value := range v.ContextValues {
// 					ctx = context.WithValue(ctx, key, value)
// 				}

// 				req = req.WithContext(ctx)
// 			}

// 			requests[k] = req
// 		}
// 	}

// 	return requests, nil
// }

// func GetFormNames(baseName string, numOfTests int) []string {
// 	formNames := make([]string, 0, numOfTests)

// 	for i := 0; i < numOfTests; i++ {
// 		name := baseName + " " + strconv.Itoa(i+1)
// 		formNames = append(formNames, name)
// 	}

// 	return formNames
// }

func validateFormTests(t *testing.T, formTest FormTestCase) {
	var validationErrors validation.Errors
	var err error

	if formTest.FormValidator == nil {
		t.Fatalf("Must Set FormValidator")
	}

	err = formTest.FormValidator.Validate(formTest.Form)

	if err != nil {
		if formTest.IsValidForm {
			//hasError = true
			t.Errorf("Should be valid form; got %s\n", err)
			fmt.Println(formTest.Form)
		}

		ok := true
		validationErrors, ok = err.(validation.Errors)

		if !ok {
			if formTest.InternalError != err.Error() {
				t.Errorf("Internal error: %s", err.Error())
			}
		} else {
			for key, expectedVal := range formTest.ValidationErrors {
				if val, ok := validationErrors[key]; ok {
					if val.Error() != expectedVal {
						//hasError = true
						t.Errorf("%s did not throw \"%s\" err \n", key, val)
					}
				} else {
					//hasError = true
					t.Errorf("Did not find %s in validation error map\n", key)
				}
			}

			if len(formTest.ValidationErrors) != len(validationErrors) {
				t.Errorf(
					"Given validation errors(%d) does not match actual error count(%d) \n",
					len(formTest.ValidationErrors),
					len(validationErrors),
				)

				keyList := make([]string, 0)

				if len(formTest.ValidationErrors) > len(validationErrors) {
					for k, v := range formTest.ValidationErrors {
						if _, ok := validationErrors[k]; !ok {
							errString := k + ": " + v + ";"
							keyList = append(keyList, errString)
						}
					}

					t.Errorf("Keys found in 'ValidationErrors' errors not in form errors - %v", keyList)

				} else {
					for k, v := range validationErrors {
						if _, ok := formTest.ValidationErrors[k]; !ok {
							errString := k + ": " + v.Error() + ";"
							keyList = append(keyList, errString)
						}
					}

					t.Errorf("Keys found in form errors not in 'ValidationErrors' errors - %v", keyList)
				}
			}
		}

	} else {
		if !formTest.IsValidForm {
			//hasError = true
			t.Errorf("Should have thrown some type of error\n")
		}
	}

	if formTest.PostExecute != nil {
		formTest.PostExecute()
	}
}
