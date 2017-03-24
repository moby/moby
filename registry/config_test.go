package registry

import (
	"strings"
	"testing"
)

func TestValidateMirror(t *testing.T) {
	valid := []string{
		"http://mirror-1.com",
		"http://mirror-1.com/",
		"https://mirror-1.com",
		"https://mirror-1.com/",
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
		"ftp://mirror-1.com",
		"http://mirror-1.com/?q=foo",
		"http://mirror-1.com/v1/",
		"http://mirror-1.com/v1/?q=foo",
		"http://mirror-1.com/v1/?q=foo#frag",
		"http://mirror-1.com?q=foo",
		"https://mirror-1.com#frag",
		"https://mirror-1.com/#frag",
		"http://foo:bar@mirror-1.com/",
		"https://mirror-1.com/v1/",
		"https://mirror-1.com/v1/#",
		"https://mirror-1.com?q",
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
			registries: []string{"http://mytest.com"},
			index:      "mytest.com",
		},
		{
			registries: []string{"https://mytest.com"},
			index:      "mytest.com",
		},
		{
			registries: []string{"HTTP://mytest.com"},
			index:      "mytest.com",
		},
		{
			registries: []string{"svn://mytest.com"},
			err:        "insecure registry svn://mytest.com should not contain '://'",
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
			registries: []string{`mytest.com:500000`},
			err:        `insecure registry mytest.com:500000 is not valid: invalid port "500000"`,
		},
		{
			registries: []string{`"mytest.com"`},
			err:        `insecure registry "mytest.com" is not valid: invalid host "\"mytest.com\""`,
		},
		{
			registries: []string{`"mytest.com:5000"`},
			err:        `insecure registry "mytest.com:5000" is not valid: invalid host "\"mytest.com"`,
		},
	}
	for _, testCase := range testCases {
		config := newServiceConfig(ServiceOptions{})
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
