package registry // import "github.com/docker/docker/registry"

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLoadAllowNondistributableArtifacts(t *testing.T) {
	testCases := []struct {
		registries []string
		cidrStrs   []string
		hostnames  []string
		err        string
	}{
		{
			registries: []string{"1.2.3.0/24"},
			cidrStrs:   []string{"1.2.3.0/24"},
		},
		{
			registries: []string{"2001:db8::/120"},
			cidrStrs:   []string{"2001:db8::/120"},
		},
		{
			registries: []string{"127.0.0.1"},
			hostnames:  []string{"127.0.0.1"},
		},
		{
			registries: []string{"127.0.0.1:8080"},
			hostnames:  []string{"127.0.0.1:8080"},
		},
		{
			registries: []string{"2001:db8::1"},
			hostnames:  []string{"2001:db8::1"},
		},
		{
			registries: []string{"[2001:db8::1]:80"},
			hostnames:  []string{"[2001:db8::1]:80"},
		},
		{
			registries: []string{"[2001:db8::1]:80"},
			hostnames:  []string{"[2001:db8::1]:80"},
		},
		{
			registries: []string{"1.2.3.0/24", "2001:db8::/120", "127.0.0.1", "127.0.0.1:8080"},
			cidrStrs:   []string{"1.2.3.0/24", "2001:db8::/120"},
			hostnames:  []string{"127.0.0.1", "127.0.0.1:8080"},
		},

		{
			registries: []string{"http://myregistry.example.com"},
			err:        "allow-nondistributable-artifacts registry http://myregistry.example.com should not contain '://'",
		},
		{
			registries: []string{"https://myregistry.example.com"},
			err:        "allow-nondistributable-artifacts registry https://myregistry.example.com should not contain '://'",
		},
		{
			registries: []string{"HTTP://myregistry.example.com"},
			err:        "allow-nondistributable-artifacts registry HTTP://myregistry.example.com should not contain '://'",
		},
		{
			registries: []string{"svn://myregistry.example.com"},
			err:        "allow-nondistributable-artifacts registry svn://myregistry.example.com should not contain '://'",
		},
		{
			registries: []string{"-invalid-registry"},
			err:        "Cannot begin or end with a hyphen",
		},
		{
			registries: []string{`mytest-.com`},
			err:        `allow-nondistributable-artifacts registry mytest-.com is not valid: invalid host "mytest-.com"`,
		},
		{
			registries: []string{`1200:0000:AB00:1234:0000:2552:7777:1313:8080`},
			err:        `allow-nondistributable-artifacts registry 1200:0000:AB00:1234:0000:2552:7777:1313:8080 is not valid: invalid host "1200:0000:AB00:1234:0000:2552:7777:1313:8080"`,
		},
		{
			registries: []string{`myregistry.example.com:500000`},
			err:        `allow-nondistributable-artifacts registry myregistry.example.com:500000 is not valid: invalid port "500000"`,
		},
		{
			registries: []string{`"myregistry.example.com"`},
			err:        `allow-nondistributable-artifacts registry "myregistry.example.com" is not valid: invalid host "\"myregistry.example.com\""`,
		},
		{
			registries: []string{`"myregistry.example.com:5000"`},
			err:        `allow-nondistributable-artifacts registry "myregistry.example.com:5000" is not valid: invalid host "\"myregistry.example.com"`,
		},
	}
	for _, testCase := range testCases {
		config := emptyServiceConfig
		err := config.LoadAllowNondistributableArtifacts(testCase.registries)
		if testCase.err == "" {
			if err != nil {
				t.Fatalf("expect no error, got '%s'", err)
			}

			var cidrStrs []string
			for _, c := range config.AllowNondistributableArtifactsCIDRs {
				cidrStrs = append(cidrStrs, c.String())
			}

			sort.Strings(testCase.cidrStrs)
			sort.Strings(cidrStrs)
			if (len(testCase.cidrStrs) > 0 || len(cidrStrs) > 0) && !reflect.DeepEqual(testCase.cidrStrs, cidrStrs) {
				t.Fatalf("expect AllowNondistributableArtifactsCIDRs to be '%+v', got '%+v'", testCase.cidrStrs, cidrStrs)
			}

			sort.Strings(testCase.hostnames)
			sort.Strings(config.AllowNondistributableArtifactsHostnames)
			if (len(testCase.hostnames) > 0 || len(config.AllowNondistributableArtifactsHostnames) > 0) && !reflect.DeepEqual(testCase.hostnames, config.AllowNondistributableArtifactsHostnames) {
				t.Fatalf("expect AllowNondistributableArtifactsHostnames to be '%+v', got '%+v'", testCase.hostnames, config.AllowNondistributableArtifactsHostnames)
			}
		} else {
			if err == nil {
				t.Fatalf("expect error '%s', got no error", testCase.err)
			}
			if !strings.Contains(err.Error(), testCase.err) {
				t.Fatalf("expect error '%s', got '%s'", testCase.err, err)
			}
		}
	}
}

func TestValidateMirror(t *testing.T) {
	valid := []string{
		"http://mirror-1.example.com",
		"http://mirror-1.example.com/",
		"https://mirror-1.example.com",
		"https://mirror-1.example.com/",
		"http://localhost",
		"https://localhost",
		"http://localhost:5000",
		"https://localhost:5000",
		"http://127.0.0.1",
		"https://127.0.0.1",
		"http://127.0.0.1:5000",
		"https://127.0.0.1:5000",
	}

	invalid := []string{
		"!invalid!://%as%",
		"ftp://mirror-1.example.com",
		"http://mirror-1.example.com/?q=foo",
		"http://mirror-1.example.com/v1/",
		"http://mirror-1.example.com/v1/?q=foo",
		"http://mirror-1.example.com/v1/?q=foo#frag",
		"http://mirror-1.example.com?q=foo",
		"https://mirror-1.example.com#frag",
		"https://mirror-1.example.com/#frag",
		"http://foo:bar@mirror-1.example.com/",
		"https://mirror-1.example.com/v1/",
		"https://mirror-1.example.com/v1/#",
		"https://mirror-1.example.com?q",
	}

	for _, address := range valid {
		if ret, err := ValidateMirror(address); err != nil || ret == "" {
			t.Errorf("ValidateMirror(`"+address+"`) got %s %s", ret, err)
		}
	}

	for _, address := range invalid {
		if ret, err := ValidateMirror(address); err == nil || ret != "" {
			t.Errorf("ValidateMirror(`"+address+"`) got %s %s", ret, err)
		}
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
		config := emptyServiceConfig
		err := config.LoadInsecureRegistries(testCase.registries)
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
			if !strings.Contains(err.Error(), testCase.err) {
				t.Fatalf("expect error '%s', got '%s'", testCase.err, err)
			}
		}
	}
}

func TestNewServiceConfig(t *testing.T) {
	testCases := []struct {
		opts   ServiceOptions
		errStr string
	}{
		{
			ServiceOptions{},
			"",
		},
		{
			ServiceOptions{
				Mirrors: []string{"example.com:5000"},
			},
			`invalid mirror: unsupported scheme "example.com" in "example.com:5000"`,
		},
		{
			ServiceOptions{
				Mirrors: []string{"http://example.com:5000"},
			},
			"",
		},
		{
			ServiceOptions{
				InsecureRegistries: []string{"[fe80::]/64"},
			},
			`insecure registry [fe80::]/64 is not valid: invalid host "[fe80::]/64"`,
		},
		{
			ServiceOptions{
				InsecureRegistries: []string{"102.10.8.1/24"},
			},
			"",
		},
		{
			ServiceOptions{
				AllowNondistributableArtifacts: []string{"[fe80::]/64"},
			},
			`allow-nondistributable-artifacts registry [fe80::]/64 is not valid: invalid host "[fe80::]/64"`,
		},
		{
			ServiceOptions{
				AllowNondistributableArtifacts: []string{"102.10.8.1/24"},
			},
			"",
		},
	}

	for _, testCase := range testCases {
		_, err := newServiceConfig(testCase.opts)
		if testCase.errStr != "" {
			assert.Check(t, is.Error(err, testCase.errStr))
		} else {
			assert.Check(t, err)
		}
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
	}
}
