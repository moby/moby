package registry // import "github.com/docker/docker/registry"

import (
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateMirror(t *testing.T) {
	tests := []struct {
		input       string
		output      string
		expectedErr string
	}{
		// Valid cases
		{
			input:  "http://mirror-1.example.com",
			output: "http://mirror-1.example.com/",
		},
		{
			input:  "http://mirror-1.example.com/",
			output: "http://mirror-1.example.com/",
		},
		{
			input:  "https://mirror-1.example.com",
			output: "https://mirror-1.example.com/",
		},
		{
			input:  "https://mirror-1.example.com/",
			output: "https://mirror-1.example.com/",
		},
		{
			input:  "http://localhost",
			output: "http://localhost/",
		},
		{
			input:  "https://localhost",
			output: "https://localhost/",
		},
		{
			input:  "http://localhost:5000",
			output: "http://localhost:5000/",
		},
		{
			input:  "https://localhost:5000",
			output: "https://localhost:5000/",
		},
		{
			input:  "http://127.0.0.1",
			output: "http://127.0.0.1/",
		},
		{
			input:  "https://127.0.0.1",
			output: "https://127.0.0.1/",
		},
		{
			input:  "http://127.0.0.1:5000",
			output: "http://127.0.0.1:5000/",
		},
		{
			input:  "https://127.0.0.1:5000",
			output: "https://127.0.0.1:5000/",
		},
		{
			input:  "http://mirror-1.example.com/v1/",
			output: "http://mirror-1.example.com/v1/",
		},
		{
			input:  "https://mirror-1.example.com/v1/",
			output: "https://mirror-1.example.com/v1/",
		},

		// Invalid cases
		{
			input:       "!invalid!://%as%",
			expectedErr: `invalid mirror: "!invalid!://%as%" is not a valid URI: parse "!invalid!://%as%": first path segment in URL cannot contain colon`,
		},
		{
			input:       "ftp://mirror-1.example.com",
			expectedErr: `invalid mirror: unsupported scheme "ftp" in "ftp://mirror-1.example.com"`,
		},
		{
			input:       "http://mirror-1.example.com/?q=foo",
			expectedErr: `invalid mirror: query or fragment at end of the URI "http://mirror-1.example.com/?q=foo"`,
		},
		{
			input:       "http://mirror-1.example.com/v1/?q=foo",
			expectedErr: `invalid mirror: query or fragment at end of the URI "http://mirror-1.example.com/v1/?q=foo"`,
		},
		{
			input:       "http://mirror-1.example.com/v1/?q=foo#frag",
			expectedErr: `invalid mirror: query or fragment at end of the URI "http://mirror-1.example.com/v1/?q=foo#frag"`,
		},
		{
			input:       "http://mirror-1.example.com?q=foo",
			expectedErr: `invalid mirror: query or fragment at end of the URI "http://mirror-1.example.com?q=foo"`,
		},
		{
			input:       "https://mirror-1.example.com#frag",
			expectedErr: `invalid mirror: query or fragment at end of the URI "https://mirror-1.example.com#frag"`,
		},
		{
			input:       "https://mirror-1.example.com/#frag",
			expectedErr: `invalid mirror: query or fragment at end of the URI "https://mirror-1.example.com/#frag"`,
		},
		{
			input:       "http://foo:bar@mirror-1.example.com/",
			expectedErr: `invalid mirror: username/password not allowed in URI "http://foo:xxxxx@mirror-1.example.com/"`,
		},
		{
			input:       "https://mirror-1.example.com/v1/#frag",
			expectedErr: `invalid mirror: query or fragment at end of the URI "https://mirror-1.example.com/v1/#frag"`,
		},
		{
			input:       "https://mirror-1.example.com?q",
			expectedErr: `invalid mirror: query or fragment at end of the URI "https://mirror-1.example.com?q"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			out, err := ValidateMirror(tc.input)
			if tc.expectedErr != "" {
				assert.Error(t, err, tc.expectedErr)
			} else {
				assert.NilError(t, err)
			}
			assert.Check(t, is.Equal(out, tc.output))
		})
	}
}

func TestLoadInsecureRegistries(t *testing.T) {
	testCases := []struct {
		registries []string
		index      string
		err        string
	}{
		{
			registries: []string{"127.0.0.1"},
			index:      "127.0.0.1",
		},
		{
			registries: []string{"127.0.0.1:8080"},
			index:      "127.0.0.1:8080",
		},
		{
			registries: []string{"2001:db8::1"},
			index:      "2001:db8::1",
		},
		{
			registries: []string{"[2001:db8::1]:80"},
			index:      "[2001:db8::1]:80",
		},
		{
			registries: []string{"http://myregistry.example.com"},
			index:      "myregistry.example.com",
		},
		{
			registries: []string{"https://myregistry.example.com"},
			index:      "myregistry.example.com",
		},
		{
			registries: []string{"HTTP://myregistry.example.com"},
			index:      "myregistry.example.com",
		},
		{
			registries: []string{"svn://myregistry.example.com"},
			err:        "insecure registry svn://myregistry.example.com should not contain '://'",
		},
		{
			registries: []string{"-invalid-registry"},
			err:        "Cannot begin or end with a hyphen",
		},
		{
			registries: []string{`mytest-.com`},
			err:        `insecure registry mytest-.com is not valid: invalid host "mytest-.com"`,
		},
		{
			registries: []string{`1200:0000:AB00:1234:0000:2552:7777:1313:8080`},
			err:        `insecure registry 1200:0000:AB00:1234:0000:2552:7777:1313:8080 is not valid: invalid host "1200:0000:AB00:1234:0000:2552:7777:1313:8080"`,
		},
		{
			registries: []string{`myregistry.example.com:500000`},
			err:        `insecure registry myregistry.example.com:500000 is not valid: invalid port "500000"`,
		},
		{
			registries: []string{`"myregistry.example.com"`},
			err:        `insecure registry "myregistry.example.com" is not valid: invalid host "\"myregistry.example.com\""`,
		},
		{
			registries: []string{`"myregistry.example.com:5000"`},
			err:        `insecure registry "myregistry.example.com:5000" is not valid: invalid host "\"myregistry.example.com"`,
		},
	}
	for _, testCase := range testCases {
		config := &serviceConfig{}
		err := config.loadInsecureRegistries(testCase.registries)
		if testCase.err == "" {
			if err != nil {
				t.Fatalf("expect no error, got '%s'", err)
			}
			match := false
			for index := range config.IndexConfigs {
				if index == testCase.index {
					match = true
				}
			}
			if !match {
				t.Fatalf("expect index configs to contain '%s', got %+v", testCase.index, config.IndexConfigs)
			}
		} else {
			if err == nil {
				t.Fatalf("expect error '%s', got no error", testCase.err)
			}
			assert.ErrorContains(t, err, testCase.err)
			assert.Check(t, cerrdefs.IsInvalidArgument(err))
		}
	}
}

func TestNewServiceConfig(t *testing.T) {
	tests := []struct {
		doc    string
		opts   ServiceOptions
		errStr string
	}{
		{
			doc: "empty config",
		},
		{
			doc: "invalid mirror",
			opts: ServiceOptions{
				Mirrors: []string{"example.com:5000"},
			},
			errStr: `invalid mirror: unsupported scheme "example.com" in "example.com:5000"`,
		},
		{
			doc: "valid mirror",
			opts: ServiceOptions{
				Mirrors: []string{"https://example.com:5000"},
			},
		},
		{
			doc: "invalid insecure registry",
			opts: ServiceOptions{
				InsecureRegistries: []string{"[fe80::]/64"},
			},
			errStr: `insecure registry [fe80::]/64 is not valid: invalid host "[fe80::]/64"`,
		},
		{
			doc: "valid insecure registry",
			opts: ServiceOptions{
				InsecureRegistries: []string{"102.10.8.1/24"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			_, err := newServiceConfig(tc.opts)
			if tc.errStr != "" {
				assert.Check(t, is.Error(err, tc.errStr))
				assert.Check(t, cerrdefs.IsInvalidArgument(err))
			} else {
				assert.Check(t, err)
			}
		})
	}
}

func TestValidateIndexName(t *testing.T) {
	valid := []struct {
		index  string
		expect string
	}{
		{
			index:  "index.docker.io",
			expect: "docker.io",
		},
		{
			index:  "example.com",
			expect: "example.com",
		},
		{
			index:  "127.0.0.1:8080",
			expect: "127.0.0.1:8080",
		},
		{
			index:  "mytest-1.com",
			expect: "mytest-1.com",
		},
		{
			index:  "mirror-1.example.com/v1/?q=foo",
			expect: "mirror-1.example.com/v1/?q=foo",
		},
	}

	for _, testCase := range valid {
		result, err := ValidateIndexName(testCase.index)
		if assert.Check(t, err) {
			assert.Check(t, is.Equal(testCase.expect, result))
		}
	}
}

func TestValidateIndexNameWithError(t *testing.T) {
	invalid := []struct {
		index string
		err   string
	}{
		{
			index: "docker.io-",
			err:   "invalid index name (docker.io-). Cannot begin or end with a hyphen",
		},
		{
			index: "-example.com",
			err:   "invalid index name (-example.com). Cannot begin or end with a hyphen",
		},
		{
			index: "mirror-1.example.com/v1/?q=foo-",
			err:   "invalid index name (mirror-1.example.com/v1/?q=foo-). Cannot begin or end with a hyphen",
		},
	}
	for _, testCase := range invalid {
		_, err := ValidateIndexName(testCase.index)
		assert.Check(t, is.Error(err, testCase.err))
		assert.Check(t, cerrdefs.IsInvalidArgument(err))
	}
}
