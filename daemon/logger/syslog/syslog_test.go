package syslog // import "github.com/docker/docker/daemon/logger/syslog"

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	syslog "github.com/RackSec/srslog"
)

func functionMatches(expectedFun interface{}, actualFun interface{}) bool {
	return reflect.ValueOf(expectedFun).Pointer() == reflect.ValueOf(actualFun).Pointer()
}

func TestParseLogFormat(t *testing.T) {
	formatter, framer, err := parseLogFormat("rfc5424", "udp")
	if err != nil || !functionMatches(rfc5424formatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse rfc5424 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424", "tcp+tls")
	if err != nil || !functionMatches(rfc5424formatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		t.Fatal("Failed to parse rfc5424 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424micro", "udp")
	if err != nil || !functionMatches(rfc5424microformatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse rfc5424 (microsecond) format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424micro", "tcp+tls")
	if err != nil || !functionMatches(rfc5424microformatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		t.Fatal("Failed to parse rfc5424 (microsecond) format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc3164", "")
	if err != nil || !functionMatches(syslog.RFC3164Formatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse rfc3164 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("", "")
	if err != nil || !functionMatches(syslog.UnixFormatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse empty format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("invalid", "")
	if err == nil {
		t.Fatal("Failed to parse invalid format", err, formatter, framer)
	}
}

func TestValidateLogOptEmpty(t *testing.T) {
	emptyConfig := make(map[string]string)
	if err := ValidateLogOpt(emptyConfig); err != nil {
		t.Fatal("Failed to parse empty config", err)
	}
}

func TestValidateSyslogAddress(t *testing.T) {
	const sockPlaceholder = "/TEMPDIR/socket.sock"
	s, err := os.Create(filepath.Join(t.TempDir(), "socket.sock"))
	if err != nil {
		log.Fatal(err)
	}
	socketPath := s.Name()
	_ = s.Close()

	tests := []struct {
		address     string
		expectedErr string
		skipOn      string
	}{
		{
			address:     "this is not an uri",
			expectedErr: "unsupported scheme: ''",
		},
		{
			address:     "corrupted:42",
			expectedErr: "unsupported scheme: 'corrupted'",
		},
		{
			address: "unix://" + sockPlaceholder,
			skipOn:  "windows", // doesn't work with unix:// sockets
		},
		{
			address:     "unix:///does_not_exist",
			expectedErr: "no such file or directory",
			skipOn:      "windows", // error message differs
		},
		{
			address: "tcp://1.2.3.4",
		},
		{
			address: "udp://1.2.3.4",
		},
		{
			address:     "http://1.2.3.4",
			expectedErr: "unsupported scheme: 'http'",
		},
	}
	for _, tc := range tests {
		tc := tc
		if tc.skipOn == runtime.GOOS {
			continue
		}
		t.Run(tc.address, func(t *testing.T) {
			address := strings.Replace(tc.address, sockPlaceholder, socketPath, 1)
			err := ValidateLogOpt(map[string]string{"syslog-address": address})
			if tc.expectedErr != "" {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected error to contain '%s', got: '%s'", tc.expectedErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: '%s'", err)
			}
		})
	}
}

func TestParseAddressDefaultPort(t *testing.T) {
	_, address, err := parseAddress("tcp://1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}

	_, port, _ := net.SplitHostPort(address)
	if port != defaultPort {
		t.Fatalf("Expected to default to port %s. It used port %s", defaultPort, port)
	}
}

func TestValidateSyslogFacility(t *testing.T) {
	err := ValidateLogOpt(map[string]string{
		"syslog-facility": "Invalid facility",
	})
	if err == nil {
		t.Fatal("Expected error if facility level is invalid")
	}
}

func TestValidateLogOptSyslogFormat(t *testing.T) {
	err := ValidateLogOpt(map[string]string{
		"syslog-format": "Invalid format",
	})
	if err == nil {
		t.Fatal("Expected error if format is invalid")
	}
}

func TestValidateLogOpt(t *testing.T) {
	err := ValidateLogOpt(map[string]string{
		"env":                    "http://127.0.0.1",
		"env-regex":              "abc",
		"labels":                 "labelA",
		"labels-regex":           "def",
		"syslog-address":         "udp://1.2.3.4:1111",
		"syslog-facility":        "daemon",
		"syslog-tls-ca-cert":     "/etc/ca-certificates/custom/ca.pem",
		"syslog-tls-cert":        "/etc/ca-certificates/custom/cert.pem",
		"syslog-tls-key":         "/etc/ca-certificates/custom/key.pem",
		"syslog-tls-skip-verify": "true",
		"tag":                    "true",
		"syslog-format":          "rfc3164",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateLogOpt(map[string]string{
		"not-supported-option": "a",
	})
	if err == nil {
		t.Fatal("Expecting error on unsupported options")
	}
}
