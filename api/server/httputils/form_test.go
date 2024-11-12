package httputils // import "github.com/docker/docker/api/server/httputils"

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBoolValue(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"no":    false,
		"false": false,
		"none":  false,
		"1":     true,
		"yes":   true,
		"true":  true,
		"one":   true,
		"100":   true,
	}

	for c, e := range cases {
		v := url.Values{}
		v.Set("test", c)
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Form = v

		a := BoolValue(r, "test")
		if a != e {
			t.Fatalf("Value: %s, expected: %v, actual: %v", c, e, a)
		}
	}
}

func TestBoolValueOrDefault(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "", nil)
	if !BoolValueOrDefault(r, "queryparam", true) {
		t.Fatal("Expected to get true default value, got false")
	}

	v := url.Values{}
	v.Set("param", "")
	r, _ = http.NewRequest(http.MethodGet, "", nil)
	r.Form = v
	if BoolValueOrDefault(r, "param", true) {
		t.Fatal("Expected not to get true")
	}
}

func TestInt64ValueOrZero(t *testing.T) {
	cases := map[string]int64{
		"":     0,
		"asdf": 0,
		"0":    0,
		"1":    1,
	}

	for c, e := range cases {
		v := url.Values{}
		v.Set("test", c)
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Form = v

		a := Int64ValueOrZero(r, "test")
		if a != e {
			t.Fatalf("Value: %s, expected: %v, actual: %v", c, e, a)
		}
	}
}

func TestInt64ValueOrDefault(t *testing.T) {
	cases := map[string]int64{
		"":   -1,
		"-1": -1,
		"42": 42,
	}

	for c, e := range cases {
		v := url.Values{}
		v.Set("test", c)
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Form = v

		a, err := Int64ValueOrDefault(r, "test", -1)
		if a != e {
			t.Fatalf("Value: %s, expected: %v, actual: %v", c, e, a)
		}
		if err != nil {
			t.Fatalf("Error should be nil, but received: %s", err)
		}
	}
}

func TestInt64ValueOrDefaultWithError(t *testing.T) {
	v := url.Values{}
	v.Set("test", "invalid")
	r, _ := http.NewRequest(http.MethodPost, "", nil)
	r.Form = v

	_, err := Int64ValueOrDefault(r, "test", -1)
	if err == nil {
		t.Fatal("Expected an error.")
	}
}

func TestUint32Value(t *testing.T) {
	const valueNotSet = "unset"

	tests := []struct {
		value       string
		expected    uint32
		expectedErr error
	}{
		{
			value:    "0",
			expected: 0,
		},
		{
			value:    strconv.FormatUint(math.MaxUint32, 10),
			expected: math.MaxUint32,
		},
		{
			value:       valueNotSet,
			expectedErr: strconv.ErrSyntax,
		},
		{
			value:       "",
			expectedErr: strconv.ErrSyntax,
		},
		{
			value:       "-1",
			expectedErr: strconv.ErrRange,
		},
		{
			value:       "4294967296", // MaxUint32+1
			expectedErr: strconv.ErrRange,
		},
		{
			value:       "not-a-number",
			expectedErr: strconv.ErrSyntax,
		},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodPost, "", nil)
			r.Form = url.Values{}
			if tc.value != valueNotSet {
				r.Form.Set("field", tc.value)
			}
			out, err := Uint32Value(r, "field")
			assert.Check(t, is.Equal(tc.expected, out))
			assert.Check(t, is.ErrorIs(err, tc.expectedErr))
		})
	}
}

func TestDecodePlatform(t *testing.T) {
	tests := []struct {
		doc          string
		platformJSON string
		expected     *ocispec.Platform
		expectedErr  string
	}{
		{
			doc:         "empty platform",
			expectedErr: `failed to parse platform: unexpected end of JSON input`,
		},
		{
			doc:          "not JSON",
			platformJSON: `linux/ams64`,
			expectedErr:  `failed to parse platform: invalid character 'l' looking for beginning of value`,
		},
		{
			doc:          "malformed JSON",
			platformJSON: `{"architecture"`,
			expectedErr:  `failed to parse platform: unexpected end of JSON input`,
		},
		{
			doc:          "missing os",
			platformJSON: `{"architecture":"amd64","os":""}`,
			expectedErr:  `both OS and Architecture must be provided`,
		},
		{
			doc:          "variant without architecture",
			platformJSON: `{"architecture":"","os":"","variant":"v7"}`,
			expectedErr:  `optional platform fields provided, but OS and Architecture are missing`,
		},
		{
			doc:          "missing architecture",
			platformJSON: `{"architecture":"","os":"linux"}`,
			expectedErr:  `both OS and Architecture must be provided`,
		},
		{
			doc:          "os.version without os and architecture",
			platformJSON: `{"architecture":"","os":"","os.version":"12.0"}`,
			expectedErr:  `optional platform fields provided, but OS and Architecture are missing`,
		},
		{
			doc:          "os.features without os and architecture",
			platformJSON: `{"architecture":"","os":"","os.features":["a","b"]}`,
			expectedErr:  `optional platform fields provided, but OS and Architecture are missing`,
		},
		{
			doc:          "valid platform",
			platformJSON: `{"architecture":"arm64","os":"linux","os.version":"12.0", "os.features":["a","b"], "variant": "v7"}`,
			expected: &ocispec.Platform{
				Architecture: "arm64",
				OS:           "linux",
				OSVersion:    "12.0",
				OSFeatures:   []string{"a", "b"},
				Variant:      "v7",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			p, err := DecodePlatform(tc.platformJSON)
			assert.Check(t, is.DeepEqual(p, tc.expected))
			if tc.expectedErr != "" {
				assert.Check(t, errdefs.IsInvalidParameter(err))
				assert.Check(t, is.Error(err, tc.expectedErr))
			} else {
				assert.Check(t, err)
			}
		})
	}
}
