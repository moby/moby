package timeutils

import (
	"strconv"
	"time"
)

// GetTimestamp tries to parse given string as RFC3339 time
// or Unix timestamp, if successful returns a Unix timestamp
// as string otherwise returns value back.
func GetTimestamp(value string) string {
	format := RFC3339NanoFixed
	loc := time.FixedZone(time.Now().Zone())
	if len(value) < len(format) {
		format = format[:len(value)]
	}
	t, err := time.ParseInLocation(format, value, loc)
	if err != nil {
		return value
	}
	return strconv.FormatInt(t.Unix(), 10)
}
