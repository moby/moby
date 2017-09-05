// +build linux

package gelf

import (
	"testing"
)

//Validate parseAddress
func TestParseAddress(t *testing.T) {
	url, err := parseAddress("udp://127.0.0.1:12201")
	if err != nil {
		t.Fatal(err)
	}
	if url.String() != "udp://127.0.0.1:12201" {
		t.Fatalf("Expected address udp://127.0.0.1:12201, got %s", url.String())
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

//Validate UDP options
func TestUDPValidateLogOpt(t *testing.T) {
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

	err = ValidateLogOpt(map[string]string{})
	if err == nil {
		t.Fatal("Expected required parameter error")
	}
}

//Validate TCP options
func TestTCPValidateLogOpt(t *testing.T) {
	err := ValidateLogOpt(map[string]string{
		"gelf-address": "tcp://127.0.0.1:12201",
	})
	if err != nil {
		t.Fatal("Expected TCP to be supported")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":           "tcp://127.0.0.1:12201",
		"gelf-compression-level": "9",
	})
	if err == nil {
		t.Fatal("Expected TCP to reject compression level")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":          "tcp://127.0.0.1:12201",
		"gelf-compression-type": "gzip",
	})
	if err == nil {
		t.Fatal("Expected TCP to reject compression type")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":             "tcp://127.0.0.1:12201",
		"gelf-tcp-max-reconnect":   "5",
		"gelf-tcp-reconnect-delay": "10",
	})
	if err != nil {
		t.Fatal("Expected TCP reconnect to be a valid parameters")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":             "tcp://127.0.0.1:12201",
		"gelf-tcp-max-reconnect":   "-1",
		"gelf-tcp-reconnect-delay": "-3",
	})
	if err == nil {
		t.Fatal("Expected negative TCP reconnect to be rejected")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":             "tcp://127.0.0.1:12201",
		"gelf-tcp-max-reconnect":   "invalid",
		"gelf-tcp-reconnect-delay": "invalid",
	})
	if err == nil {
		t.Fatal("Expected TCP reconnect to be required to be an int")
	}

	err = ValidateLogOpt(map[string]string{
		"gelf-address":             "udp://127.0.0.1:12201",
		"gelf-tcp-max-reconnect":   "1",
		"gelf-tcp-reconnect-delay": "3",
	})
	if err == nil {
		t.Fatal("Expected TCP reconnect to be invalid for UDP")
	}
}
