package confutil

import (
	"io/ioutil"
	"os"

	_ "github.com/lib/pq"

	yaml "gopkg.in/yaml.v2"
)

const (
	// DateTimeLayout is global format for date time
	DateTimeLayout = "2006-01-02 15:04:05"

	// DateLayout is global format for date
	DateLayout = "2006-01-02"

	// PostgresDateLayout is date format used when receiving time from database
	PostgresDateLayout = "2006-01-02T15:04:05Z"

	// FormDateTimeLayout is format that should be received from a form
	FormDateTimeLayout = "01/02/2006 3:04PM"

	// FormDateLayout is format that should be received from a form
	FormDateLayout = "01/02/2006"

	// HashPassword decodes to "currentpassword"
	HashPassword = "$2a$10$Olu8gAjliUFT4rU1Xe6kz.FI3qWvEyXeTUWCI9k196z6.rPB44t5K"
)

func ConfigSettings(envString string) *Settings {
	var settings *Settings
	configFile := os.Getenv(envString)
	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(source, &settings)
	if err != nil {
		panic(err)
	}

	return settings
}
