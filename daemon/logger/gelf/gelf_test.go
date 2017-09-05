// +build linux

package gelf

import (
	"testing"
)

//Validate parseAddress
func TestParseAddress(t *testing.T) {
	host, err := parseAddress("udp://127.0.0.1:12201")
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1:12201" {
		t.Fatalf("Expected host 127.0.0.1, got %s", host)
	}

	_, err = parseAddress("127.0.0.1:12201")
	if err == nil {
		t.Fatal("Expected error requiring protocol")
	}

	_, err = parseAddress("http://127.0.0.1:12201")
	if err == nil {
		t.Fatal("Expected error restricting protocol")
	}
}

//Validate options
func TestValidateLogOpt(t *testing.T) {
	err := ValidateLogOpt(map[string]string{
		"gelf-address":           "udp://127.0.0.1:12201",
		"tag":                    "testtag",
		"labels":                 "testlabel",
		"env":                    "testenv",
		"env-regex":              "testenv-regex",
		"gelf-compression-level": "9",
		"gelf-compression-type":  "gzip",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":           "udp://127.0.0.1:12201",
		"gelf-compression-level": "ultra",
		"gelf-compression-type":  "zlib",
	})
	if err == nil {
		t.Fatal("Expected compression level error")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":          "udp://127.0.0.1:12201",
		"gelf-compression-type": "rar",
	})
	if err == nil {
		t.Fatal("Expected compression type error")
	}

	err = ValidateLogOpt(map[string]string{
		"invalid": "invalid",
	})
	if err == nil {
		t.Fatal("Expected unknown option error")
	}
}
