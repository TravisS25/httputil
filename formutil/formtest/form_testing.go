package formtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/mux"

	"github.com/TravisS25/httputil/formutil"

	validation "github.com/go-ozzo/ozzo-validation"
)

type FormRequestConfig struct {
	// TestName is the name of current test - Required
	TestName string

	// Method is http method to use for request - Optional
	Method string

	// URL is uri you wish to request - Optional
	URL string

	// Validatable is used for forms that validate themselves, generally inner forms - Optional
	Validatable validation.Validatable

	// Validate is interface for struct that will validate form - Optional
	Validator formutil.RequestValidator

	// Form is form values to use to inject into request - Required
	Form interface{}

	// Instance is instance of a model in which a form might need, usually
	// on an edit request - Optional
	Instance interface{}

	// RouterValues is used to inject router variables into the request - Optional
	RouterValues map[string]string

	// ContextValues are context#Context to use for request - Optional
	ContextValues map[interface{}]interface{}

	// PostExecute can be used to exec some logic that you may need to run inbetween test cases
	// such as clean up logic before the next test is run - Optional
	PostExecute func(form interface{})

	// ValidationErrors is a map of what errors you expect to return from test
	// The key is the json name of the field and value is the error message the
	// field should return - Optional
	ValidationErrors map[string]interface{}

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

func formValidation(t *testing.T, mapKey string, formValidationErr error, expectedErr interface{}) error {
	var err error

	if innerExpectedErr, k := expectedErr.(map[string]interface{}); k {
		if innerFormErr, j := formValidationErr.(validation.Errors); j {
			for innerExpectedKey := range innerExpectedErr {
				if innerFormVal, ok := innerFormErr[innerExpectedKey]; ok {
					innerExpectedVal := innerExpectedErr[innerExpectedKey]

					switch innerExpectedVal.(type) {
					case map[string]interface{}:
						//fmt.Printf("map val switch\n")
						err = formValidation(t, innerExpectedKey, innerFormVal, innerExpectedVal)

						if err != nil {
							//fmt.Printf("form err\n")
							return err
						}
					case string:
						//fmt.Printf("string val switch\n")

						if len(innerExpectedErr) != len(innerFormErr) {
							if len(innerExpectedErr) > len(innerFormErr) {
								for k := range innerExpectedErr {
									if _, ok := innerFormErr[k]; !ok {
										t.Errorf("form testing: Key \"%s\" found in \"ValidationErrors\" that is not in form errors", k)
									}
								}
							} else {
								for k, v := range innerFormErr {
									if _, ok := innerExpectedErr[k]; !ok {
										//t.Errorf("heeeey type: %v", validationErrors["invoiceItems"]["0"])
										t.Errorf("form testing: Key \"%s\" found in form errors that is not in \"ValidationErrors\"\n  Key \"%s\" threw err: %s\n", k, k, v.Error())
									}
								}
							}
						}

						if innerFormVal.Error() != innerExpectedVal {
							t.Errorf(
								"form testing: Key \"%s\" threw err: \"%s\" \n expected: \"%s\" \n",
								innerExpectedKey,
								innerFormVal.Error(),
								innerExpectedVal,
							)
						}
					default:
						message := fmt.Sprintf("form testing: Passed \"ValidationErrors\" has unexpected type\n")
						return errors.New(message)
					}
				} else {
					t.Errorf("form testing: Key \"%s\" was in \"ValidationErrors\" but not form errors\n", innerExpectedKey)
				}
			}
		} else {
			message := fmt.Sprintf(
				"form testing: \"ValidationErrors\" error for key \"%s\" was type map but form error was not\n.  Error thrown: %s", mapKey, formValidationErr,
			)
			return errors.New(message)
		}
	} else {
		//fmt.Printf("made to non map\n")
		if formValidationErr.Error() != expectedErr {
			t.Errorf("form testing: Key \"%s\" threw err: \"%s\" \n expected: \"%s\" \n", mapKey, formValidationErr.Error(), expectedErr)
		}
	}

	return nil
}

func RunRequestFormTests(t *testing.T, deferFunc func() error, formTests []FormRequestConfig) {
	for _, formTest := range formTests {
		if formTest.TestName == "" {
			t.Fatalf("TestName required")
		}
		if formTest.Validatable == nil && formTest.Validator == nil {
			t.Fatalf("Validatable or Validator is required")
		}
		if formTest.Method == "" {
			formTest.Method = http.MethodGet
		}
		if formTest.URL == "" {
			formTest.URL = "/url"
		}

		t.Run(formTest.TestName, func(t *testing.T) {
			var formErr error
			var form interface{}

			panicked := true
			defer func() {
				if deferFunc != nil && panicked {
					err := deferFunc()

					if err != nil {
						fmt.Printf("deferFunc: " + err.Error())
					}
				}
			}()

			if formTest.Validatable != nil {
				formErr = formTest.Validatable.Validate()
			} else {
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

				req = mux.SetURLVars(req, formTest.RouterValues)
				req = mux.SetCurrentRoute(req, formTest.URL)
				form, formErr = formTest.Validator.Validate(req, formTest.Instance)
			}

			if formErr == nil {
				if formTest.ValidationErrors != nil {
					t.Errorf("Form has no errors, but 'ValidationErrors' was passed\n")
				}
			} else {
				if validationErrors, ok := formErr.(validation.Errors); ok {
					//fmt.Printf("validation err: %v\n", validationErrors)

					for key, expectedVal := range formTest.ValidationErrors {
						if fErr, valid := validationErrors[key]; valid {
							err := formValidation(t, key, fErr, expectedVal)

							if err != nil {
								t.Errorf(err.Error())
							}
						} else {
							t.Errorf("Key \"%s\" found in \"ValidationErrors\" that is not in form errors\n\n", key)
						}
					}

					for k, v := range validationErrors {
						if fErr, valid := formTest.ValidationErrors[k]; valid {
							err := formValidation(t, k, v, fErr)

							if err != nil {
								t.Errorf(err.Error())
							}
						} else {
							t.Errorf(
								"Key \"%s\" found in form errors that is not in \"ValidationErrors\"\n  Threw err: %s\n\n",
								k,
								v.Error(),
							)
						}
					}
				} else {
					if formTest.InternalError != formErr.Error() {
						t.Errorf("Internal Error: %s\n", formErr.Error())
					}
				}
			}

			if formTest.PostExecute != nil {
				formTest.PostExecute(form)
			}

			panicked = false
		})
	}
}

func RunFormTests(t *testing.T, formTests []FormTestCase) {
	for _, formTest := range formTests {
		t.Run(formTest.TestName, func(t *testing.T) {
			validateFormTests(t, formTest)
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
