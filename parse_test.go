package nat

import "testing"

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		doc      string
		input    string
		expBegin uint64
		expEnd   uint64
		expErr   string
	}{
		{
			doc:    "empty value",
			expErr: `empty string specified for ports`,
		},
		{
			doc:      "single port",
			input:    "1234",
			expBegin: 1234,
			expEnd:   1234,
		},
		{
			doc:      "single port range",
			input:    "1234-1234",
			expBegin: 1234,
			expEnd:   1234,
		},
		{
			doc:      "two port range",
			input:    "1234-1235",
			expBegin: 1234,
			expEnd:   1235,
		},
		{
			doc:      "large range",
			input:    "8000-9000",
			expBegin: 8000,
			expEnd:   9000,
		},
		{
			doc:   "zero port",
			input: "0",
		},
		{
			doc:   "zero range",
			input: "0-0",
		},
		// invalid cases
		{
			doc:    "non-numeric port",
			input:  "asdf",
			expErr: `strconv.ParseUint: parsing "asdf": invalid syntax`,
		},
		{
			doc:    "reversed range",
			input:  "9000-8000",
			expErr: `invalid range specified for port: 9000-8000`,
		},
		{
			doc:    "range missing end",
			input:  "8000-",
			expErr: `strconv.ParseUint: parsing "": invalid syntax`,
		},
		{
			doc:    "range missing start",
			input:  "-9000",
			expErr: `strconv.ParseUint: parsing "": invalid syntax`,
		},
		{
			doc:    "invalid range end",
			input:  "8000-a",
			expErr: `strconv.ParseUint: parsing "a": invalid syntax`,
		},
		{
			doc:    "invalid range end port",
			input:  "8000-9000a",
			expErr: `strconv.ParseUint: parsing "9000a": invalid syntax`,
		},
		{
			doc:    "range range start",
			input:  "a-9000",
			expErr: `strconv.ParseUint: parsing "a": invalid syntax`,
		},
		{
			doc:    "range range start port",
			input:  "8000a-9000",
			expErr: `strconv.ParseUint: parsing "8000a": invalid syntax`,
		},
		{
			doc:    "range with trailing hyphen",
			input:  "-8000-",
			expErr: `strconv.ParseUint: parsing "": invalid syntax`,
		},
		{
			doc:    "range without ports",
			input:  "-",
			expErr: `strconv.ParseUint: parsing "": invalid syntax`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			begin, end, err := ParsePortRange(tc.input)
			if tc.expErr == "" {
				if err != nil {
					t.Error(err)
				}
			} else {
				if err == nil || err.Error() != tc.expErr {
					t.Errorf("expected error '%s', got '%v'", tc.expErr, err)
				}
			}
			if begin != tc.expBegin {
				t.Errorf("expected begin %d, got %d", tc.expBegin, begin)
			}
			if end != tc.expEnd {
				t.Errorf("expected end %d, got %d", tc.expEnd, end)
			}
		})
	}
}
