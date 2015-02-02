package timeutils

import (
	"errors"
	"time"
	"fmt"
	"strconv"
)

const (
	// RFC3339NanoFixed is our own version of RFC339Nano because we want one
	// that pads the nano seconds part with zeros to ensure
	// the timestamps are aligned in the logs.
	RFC3339NanoFixed           = "2006-01-02T15:04:05.000000000Z07:00"
	RFC3339NanoFixedWithSpaces = "2006-01-02 15:04:05.000000000 07:00"
	// JSONFormat is the format used by FastMarshalJSON
	JSONFormat = `"` + time.RFC3339Nano + `"`
)

// FastMarshalJSON avoids one of the extra allocations that
// time.MarshalJSON is making.
func FastMarshalJSON(t time.Time) (string, error) {
	if y := t.Year(); y < 0 || y >= 10000 {
		// RFC 3339 is clear that years are 4 digits exactly.
		// See golang.org/issue/4556#c15 for more discussion.
		return "", errors.New("time.MarshalJSON: year outside of range [0,9999]")
	}
	return t.Format(JSONFormat), nil
}

func AcceptedFormats() []string {
	return []string{
		RFC3339NanoFixed,
		RFC3339NanoFixedWithSpaces,
		JSONFormat,
	}
}

func ConvertFrom(value string) (error error, convertValue string){
	loc := time.FixedZone(time.Now().Zone())
	for _, format := range AcceptedFormats() {
		if len(value) < len(format) {
			format = format[:len(value)]
		}
		if t, err := time.ParseInLocation(format, value, loc); err == nil {
			convertValue = strconv.FormatInt(t.Unix(), 10)
			break
		}
	}
	if convertValue == "" {
		//last try is UintTime64
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return fmt.Errorf("Invalid timestamp format on `%s` field", value), ""
		} else {
			convertValue = value
		}
	}
	return nil, convertValue
}
