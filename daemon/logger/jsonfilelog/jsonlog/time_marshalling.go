package jsonlog

import (
	"time"

	"github.com/pkg/errors"
)

const jsonFormat = `"` + time.RFC3339Nano + `"`

// fastTimeMarshalJSON avoids one of the extra allocations that
// time.MarshalJSON is making.
func fastTimeMarshalJSON(t time.Time) (string, error) {
	b, err := appendTimeJSON(nil, t)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// appendTimeJSON appends the JSON form of t to dst.
func appendTimeJSON(dst []byte, t time.Time) ([]byte, error) {
	if y := t.Year(); y < 0 || y >= 10000 {
		// RFC 3339 is clear that years are 4 digits exactly.
		// See golang.org/issue/4556#c15 for more discussion.
		return dst, errors.New("time.MarshalJSON: year outside of range [0,9999]")
	}
	return t.AppendFormat(dst, jsonFormat), nil
}
