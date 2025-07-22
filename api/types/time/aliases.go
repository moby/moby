package time

import (
	"time"

	apitype "github.com/moby/moby/api/types/time"
)

// GetTimestamp tries to parse given string as golang duration,
// then RFC3339 time and finally as a Unix timestamp. If
// any of these were successful, it returns a Unix timestamp
// as string otherwise returns the given value back.
// In case of duration input, the returned timestamp is computed
// as the given reference time minus the amount of the duration.
func GetTimestamp(value string, reference time.Time) (string, error) {
	return apitype.GetTimestamp(value, reference)
}

// ParseTimestamps returns seconds and nanoseconds from a timestamp that has
// the format ("%d.%09d", time.Unix(), int64(time.Nanosecond())).
// If the incoming nanosecond portion is longer than 9 digits it is truncated.
// The expectation is that the seconds and nanoseconds will be used to create a
// time variable.  For example:
//
//	seconds, nanoseconds, _ := ParseTimestamp("1136073600.000000001",0)
//	since := time.Unix(seconds, nanoseconds)
//
// returns seconds as defaultSeconds if value == ""
func ParseTimestamps(value string, defaultSeconds int64) (seconds int64, nanoseconds int64, _ error) {
	return apitype.ParseTimestamps(value, defaultSeconds)
}
