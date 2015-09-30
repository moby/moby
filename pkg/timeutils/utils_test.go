package timeutils

import (
	"fmt"
	"testing"
	"time"
)

func TestGetTimestamp(t *testing.T) {
	now := time.Now()
	cases := []struct{ in, expected string }{
		// Partial RFC3339 strings get parsed with second precision
		{"2006-01-02T15:04:05.999999999+07:00", "1136189045.999999999"},
		{"2006-01-02T15:04:05.999999999Z", "1136214245.999999999"},
		{"2006-01-02T15:04:05.999999999", "1136214245.999999999"},
		{"2006-01-02T15:04:05Z", "1136214245.000000000"},
		{"2006-01-02T15:04:05", "1136214245.000000000"},
		{"2006-01-02T15:04:0Z", ""},
		{"2006-01-02T15:04:0", ""},
		{"2006-01-02T15:04Z", "1136214240.000000000"},
		{"2006-01-02T15:04+00:00", "1136214240.000000000"},
		{"2006-01-02T15:04-00:00", "1136214240.000000000"},
		{"2006-01-02T15:04", "1136214240.000000000"},
		{"2006-01-02T15:0Z", ""},
		{"2006-01-02T15:0", ""},
		{"2006-01-02T15Z", "1136214000.000000000"},
		{"2006-01-02T15+00:00", "1136214000.000000000"},
		{"2006-01-02T15-00:00", "1136214000.000000000"},
		{"2006-01-02T15", "1136214000.000000000"},
		{"2006-01-02T1Z", "1136163600.000000000"},
		{"2006-01-02T1", "1136163600.000000000"},
		{"2006-01-02TZ", ""},
		{"2006-01-02T", ""},
		{"2006-01-02+00:00", "1136160000.000000000"},
		{"2006-01-02-00:00", "1136160000.000000000"},
		{"2006-01-02-00:01", "1136160060.000000000"},
		{"2006-01-02Z", "1136160000.000000000"},
		{"2006-01-02", "1136160000.000000000"},
		{"2015-05-13T20:39:09Z", "1431549549.000000000"},

		// unix timestamps returned as is
		{"1136073600", "1136073600"},
		{"1136073600.000000001", "1136073600.000000001"},

		// Durations
		{"1m", fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix())},
		{"1.5h", fmt.Sprintf("%d", now.Add(-90*time.Minute).Unix())},
		{"1h30m", fmt.Sprintf("%d", now.Add(-90*time.Minute).Unix())},

		// String fallback
		{"invalid", "invalid"},
	}

	for _, c := range cases {
		o, err := GetTimestamp(c.in, now)
		if o != c.expected {
			if err != nil {
				t.Fatalf("wrong value for '%s'. expected:'%s' got:'%s' with error: `%s`", c.in, c.expected, o, err)
			}
			t.Fatalf("wrong value for '%s'. expected:'%s' got:'%s' ", c.in, c.expected, o)
		}
	}
}

func TestParseTimestamps(t *testing.T) {
	cases := []struct {
		in                        string
		def, expectedS, expectedN int64
	}{
		// unix timestamps
		{"1136073600", 0, 1136073600, 0},
		{"1136073600.000000001", 0, 1136073600, 1},
		{"1136073600.0000000010", 0, 1136073600, 1},
		{"1136073600.00000001", 0, 1136073600, 10},
		{"foo.bar", 0, 0, 0},
		{"1136073600.bar", 0, 1136073600, 0},
		{"", -1, -1, 0},
	}

	for _, c := range cases {
		s, n, err := ParseTimestamps(c.in, c.def)
		if s != c.expectedS || n != c.expectedN {
			if err != nil {
				t.Fatalf("wrong value for input `%s` with default `%d` expected:'%d's and `%d`n got:'%d's and `%d`n with error: `%s`", c.in, c.def, c.expectedS, c.expectedN, s, n, err)
			}
		}
	}
}
