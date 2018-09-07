package timeutil

import "testing"

func TestGetCurrentLocalDateInUTC(t *testing.T) {
	time, _ := GetCurrentLocalDateInUTC("America/New_York")
	t.Errorf("time: %s", time.String())
}
