package formutil

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
)

// FormTestCase is config struct for function RunFormTests
type FormTestCase struct {
	// TestName is the name of current test
	TestName string

	// ValidationErrors is a map of what errors you expect to return from test
	// The key is the json name of the field and value is the error message the
	// field should return
	ValidationErrors map[string]string

	// Validator is interface that is responsible for validating "Form" variable
	FormValidator Validator

	// Value that will be validated against.  Whatever struct that implements
	// Validator should properly cast this to whatever form you are validating
	Form interface{}

	// IsValidForm tells RunFormTests whether or not you expect the form to
	// have any errors returned or not
	IsValidForm bool
}

func RunFormTests(t *testing.T, formTests []FormTestCase) {
	for _, formTest := range formTests {
		t.Run(formTest.TestName, func(t *testing.T) {
			// if formTest.BeforeFunc != nil {
			// 	formTest.BeforeFunc()
			// }

			err := formTest.FormValidator.Validate(formTest.Form)

			if err != nil {
				if formTest.IsValidForm {
					t.Errorf("Should be valid form; got %s\n", err)
				}

				validationErrors := err.(validation.Errors)

				// if formTest.NumOfErrors != 0 {
				// 	if len(validationErrors) != formTest.NumOfErrors {
				// 		t.Errorf(
				// 			"Should have %d errors; got %d\n",
				// 			formTest.NumOfErrors,
				// 			len(validationErrors),
				// 		)

				// 		t.Errorf("Thrown Errors: \n")

				// 		for k, v := range validationErrors {
				// 			t.Errorf("%s: %s\n", k, v)
				// 		}

				// 	}
				// } else {
				// 	t.Fatalf(`"NumOfErrors" can't be 0 with errors`)
				// }

				for key, expectedVal := range formTest.ValidationErrors {
					if val, ok := validationErrors[key]; ok {
						if val.Error() != expectedVal {
							t.Errorf("%s did not throw \"%s\" err \n", key, val)
						}
					} else {
						t.Errorf("Did not find %s in validation error map\n", key)
					}
				}

			} else {
				if !formTest.IsValidForm {
					t.Errorf("Should have thrown some type of error\n")
				}
			}

			// if formTest.AfterFunc != nil {
			// 	formTest.AfterFunc()
			// }
		})
	}
}
