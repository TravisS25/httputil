package timeutil

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TravisS25/httputil/confutil"
)

func GetCurrentDateTimeInUTC() *time.Time {
	currentDate := time.Now()
	year := strconv.Itoa(currentDate.Year())
	month := fmt.Sprintf("%02d", currentDate.Month())
	day := fmt.Sprintf("%02d", currentDate.Day())
	currentDateString := year + "-" + month + "-" + day
	currentUTCDate, _ := time.Parse(confutil.DateLayout, currentDateString)
	return &currentUTCDate
}

func GetCurrentLocalDateTimeInUTC(timeZone string) (*time.Time, error) {
	location, err := time.LoadLocation(timeZone)

	if err != nil {
		return nil, err
	}

	utc, err := time.LoadLocation("UTC")

	if err != nil {
		return nil, err
	}

	localTime := time.Now().In(location)
	utcTime := time.Date(
		localTime.Year(),
		localTime.Month(),
		localTime.Day(),
		localTime.Hour(),
		localTime.Minute(),
		localTime.Second(),
		localTime.Nanosecond(),
		utc,
	)

	return &utcTime, nil
}

func GetCurrentLocalDateInUTC(timeZone string) (*time.Time, error) {
	location, err := time.LoadLocation(timeZone)

	if err != nil {
		return nil, err
	}

	utc, err := time.LoadLocation("UTC")

	if err != nil {
		return nil, err
	}

	localTime := time.Now().In(location)
	utcTime := time.Date(
		localTime.Year(),
		localTime.Month(),
		localTime.Day(),
		0,
		0,
		0,
		0,
		utc,
	)

	return &utcTime, nil
}
