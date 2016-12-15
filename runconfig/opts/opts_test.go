package opts

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestValidateAttach(t *testing.T) {
	valid := []string{
		"stdin",
		"stdout",
		"stderr",
		"STDIN",
		"STDOUT",
		"STDERR",
	}
	if _, err := ValidateAttach("invalid"); err == nil {
		t.Fatalf("Expected error with [valid streams are STDIN, STDOUT and STDERR], got nothing")
	}

	for _, attach := range valid {
		value, err := ValidateAttach(attach)
		if err != nil {
			t.Fatal(err)
		}
		if value != strings.ToLower(attach) {
			t.Fatalf("Expected [%v], got [%v]", attach, value)
		}
	}
}

func TestValidateEnv(t *testing.T) {
	valids := map[string]string{
		"a":                   "a",
		"something":           "something",
		"_=a":                 "_=a",
		"env1=value1":         "env1=value1",
		"_env1=value1":        "_env1=value1",
		"env2=value2=value3":  "env2=value2=value3",
		"env3=abc!qwe":        "env3=abc!qwe",
		"env_4=value 4":       "env_4=value 4",
		"PATH":                fmt.Sprintf("PATH=%v", os.Getenv("PATH")),
		"PATH=something":      "PATH=something",
		"asd!qwe":             "asd!qwe",
		"1asd":                "1asd",
		"123":                 "123",
		"some space":          "some space",
		"  some space before": "  some space before",
		"some space after  ":  "some space after  ",
	}
	// Environment variables are case in-sensitive on Windows
	if runtime.GOOS == "windows" {
		valids["PaTh"] = fmt.Sprintf("PaTh=%v", os.Getenv("PATH"))
	}
	for value, expected := range valids {
		actual, err := ValidateEnv(value)
		if err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Fatalf("Expected [%v], got [%v]", expected, actual)
		}
	}
}

func TestValidateExtraHosts(t *testing.T) {
	valid := []string{
		`myhost:192.168.0.1`,
		`thathost:10.0.2.1`,
		`anipv6host:2003:ab34:e::1`,
		`ipv6local:::1`,
	}

	invalid := map[string]string{
		`myhost:192.notanipaddress.1`:  `invalid IP`,
		`thathost-nosemicolon10.0.0.1`: `bad format`,
		`anipv6host:::::1`:             `invalid IP`,
		`ipv6local:::0::`:              `invalid IP`,
	}

	for _, extrahost := range valid {
		if _, err := ValidateExtraHost(extrahost); err != nil {
			t.Fatalf("ValidateExtraHost(`"+extrahost+"`) should succeed: error %v", err)
		}
	}

	for extraHost, expectedError := range invalid {
		if _, err := ValidateExtraHost(extraHost); err == nil {
			t.Fatalf("ValidateExtraHost(`%q`) should have failed validation", extraHost)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("ValidateExtraHost(`%q`) error should contain %q", extraHost, expectedError)
			}
		}
	}
}

func TestValidateMACAddress(t *testing.T) {
	if _, err := ValidateMACAddress(`92:d0:c6:0a:29:33`); err != nil {
		t.Fatalf("ValidateMACAddress(`92:d0:c6:0a:29:33`) got %s", err)
	}

	if _, err := ValidateMACAddress(`92:d0:c6:0a:33`); err == nil {
		t.Fatalf("ValidateMACAddress(`92:d0:c6:0a:33`) succeeded; expected failure on invalid MAC")
	}

	if _, err := ValidateMACAddress(`random invalid string`); err == nil {
		t.Fatalf("ValidateMACAddress(`random invalid string`) succeeded; expected failure on invalid MAC")
	}
}
