package timestamp_test

import (
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/internal/timestamp"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParse(t *testing.T) {
	now := time.Date(2020, 1, 2, 3, 4, 5, 123456789, time.UTC)
	tests := []struct {
		in          string
		expected    string // RFC3339Nano in UTC
		expectedErr bool
	}{
		// Partial RFC3339 strings get parsed with second precision
		{in: "2006-01-02T15:04:05.999999999+07:00", expected: "2006-01-02T08:04:05.999999999Z"},
		{in: "2006-01-02T15:04:05.999999999Z", expected: "2006-01-02T15:04:05.999999999Z"},
		{in: "2006-01-02T15:04:05.999999999", expected: "2006-01-02T15:04:05.999999999Z"},
		{in: "2006-01-02T15:04:05Z", expected: "2006-01-02T15:04:05Z"},
		{in: "2006-01-02T15:04:05", expected: "2006-01-02T15:04:05Z"},
		{in: "2006-01-02T15:04:0Z", expectedErr: true},
		{in: "2006-01-02T15:04:0", expectedErr: true},
		{in: "2006-01-02T15:04Z", expected: "2006-01-02T15:04:00Z"},
		{in: "2006-01-02T15:04+00:00", expected: "2006-01-02T15:04:00Z"},
		{in: "2006-01-02T15:04-00:00", expected: "2006-01-02T15:04:00Z"},
		{in: "2006-01-02T15:04", expected: "2006-01-02T15:04:00Z"},
		{in: "2006-01-02T15:0Z", expectedErr: true},
		{in: "2006-01-02T15:0", expectedErr: true},
		{in: "2006-01-02T15Z", expected: "2006-01-02T15:00:00Z"},
		{in: "2006-01-02T15+00:00", expected: "2006-01-02T15:00:00Z"},
		{in: "2006-01-02T15-00:00", expected: "2006-01-02T15:00:00Z"},
		{in: "2006-01-02T15", expected: "2006-01-02T15:00:00Z"},
		{in: "2006-01-02T1Z", expected: "2006-01-02T01:00:00Z"},
		{in: "2006-01-02T1", expected: "2006-01-02T01:00:00Z"},
		{in: "2006-01-02TZ", expectedErr: true},
		{in: "2006-01-02T", expectedErr: true},
		{in: "2006-01-02+00:00", expected: "2006-01-02T00:00:00Z"},
		{in: "2006-01-02-00:00", expected: "2006-01-02T00:00:00Z"},
		{in: "2006-01-02-00:01", expected: "2006-01-02T00:01:00Z"},
		{in: "2006-01-02Z", expected: "2006-01-02T00:00:00Z"},
		{in: "2006-01-02", expected: "2006-01-02T00:00:00Z"},
		{in: "2015-05-13T20:39:09Z", expected: "2015-05-13T20:39:09Z"},

		// Unix timestamps
		{in: "1136073600", expected: "2006-01-01T00:00:00Z"},
		{in: "1136073600.000000001", expected: "2006-01-01T00:00:00.000000001Z"},

		// Durations (relative to `now`)
		{in: "1m", expected: "2020-01-02T03:03:05.123456789Z"},
		{in: "1.5h", expected: "2020-01-02T01:34:05.123456789Z"},
		{in: "1h30m", expected: "2020-01-02T01:34:05.123456789Z"},

		// invalid values
		{in: " 1136073600 \t", expectedErr: true},
		{in: "foo.bar", expectedErr: true},
		{in: "1136073600.bar", expectedErr: true},
		{in: "invalid", expectedErr: true},
		{in: "", expectedErr: true},
	}

	for _, tc := range tests {
		name := tc.in
		if name == "" {
			name = "<empty>"
		}
		t.Run(name, func(t *testing.T) {
			out, err := timestamp.Parse(tc.in, now)
			if tc.expectedErr {
				assert.Assert(t, err != nil, "expected error for %q, got none", tc.in)
				return
			}
			assert.NilError(t, err)

			want, err := time.Parse(time.RFC3339Nano, tc.expected)
			assert.NilError(t, err, "invalid expected value")

			assert.Assert(t, out.Equal(want),
				"expected: %s\ngot:      %s",
				want.Format(time.RFC3339Nano),
				out.Format(time.RFC3339Nano),
			)
		})
	}
}

func TestParseUnixTimestamp(t *testing.T) {
	tests := []struct {
		in          string
		expectedS   int64
		expectedN   int64
		expectedErr bool
	}{
		// unix timestamps
		{in: "1136073600", expectedS: 1136073600, expectedN: 0},
		{in: "1136073600.", expectedS: 1136073600, expectedN: 0}, // allow empty nanoseconds
		{in: "1136073600.0", expectedS: 1136073600, expectedN: 0},
		{in: "1136073600.000000001", expectedS: 1136073600, expectedN: 1},
		{in: "1136073600.0000000010", expectedS: 1136073600, expectedN: 1}, // truncates
		{in: "1136073600.0000000001", expectedS: 1136073600, expectedN: 0}, // truncates
		{in: "1136073600.0000000009", expectedS: 1136073600, expectedN: 0}, // truncates
		{in: "1136073600.00000001", expectedS: 1136073600, expectedN: 10},  // pads
		{in: "1136073600.1", expectedS: 1136073600, expectedN: 100000000},  // pads

		// invalid values
		{in: " 1136073600 \t", expectedErr: true},
		{in: "foo.bar", expectedErr: true},
		{in: ".0000000009", expectedErr: true},
		{in: "1136073600.bar", expectedErr: true},

		// empty value
		{in: ""},
	}

	for _, tc := range tests {
		name := tc.in
		if name == "" {
			name = "<empty>"
		}
		t.Run(name, func(t *testing.T) {
			out, err := timestamp.ParseUnixTimestamp(tc.in)
			if tc.expectedErr {
				assert.Assert(t, err != nil, "expected error for %q, got none", tc.in)
				return
			}
			assert.NilError(t, err)
			if tc.in == "" {
				assert.Assert(t, out.IsZero())
				return
			}
			assert.Check(t, is.Equal(out.Unix(), tc.expectedS))
			assert.Check(t, is.Equal(int64(out.Nanosecond()), tc.expectedN))
		})
	}
}
