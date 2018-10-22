package formtest

import (
	"fmt"
	"testing"

	"github.com/TravisS25/httputil/formutil"

	validation "github.com/go-ozzo/ozzo-validation"
)

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

	// Value that will be validated against.  Whatever struct that implements
	// Validator should properly cast this to whatever form you are validating - Required
	Form interface{}

	// IsValidForm tells RunFormTests whether or not you expect the form to
	// have any errors returned or not - Optional
	IsValidForm bool

	// NumOfErrors determines how many errors a user expects the form to return
	// Should be the same number of entries in the "ValidationErrors" map variable but
	// adding NumOfErrors is a confirmation
	// If NumOfErrors does not match count of actual errors, test will fail - Optional
	NumOfErrors int

	// PostExecute can be used to exec some logic that you may need to run inbetween test cases
	// such as clean up logic before the next test is run - Optional
	PostExecute func()

	InternalError string
}

func RunFormTests(t *testing.T, formTests []FormTestCase) {
	for _, formTest := range formTests {
		t.Run(formTest.TestName, func(t *testing.T) {
			ok := true
			var validationErrors validation.Errors
			err := formTest.FormValidator.Validate(formTest.Form)

			if err != nil {
				if formTest.IsValidForm {
					//hasError = true
					t.Errorf("Should be valid form; got %s\n", err)
					fmt.Println(formTest.Form)
				}

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

						// t.Errorf("Given errors - %s\n", formTest.ValidationErrors)
						// t.Errorf("Form errors - %s\n", validationErrors)
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

			//panic("hello panic")
			// if hasError && validationErrors != nil {
			// 	fmt.Printf("Actual errors: %s\n", validationErrors)
			// }
		})
	}
}
