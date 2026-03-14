package syslog

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
	"github.com/moby/moby/v2/daemon/logger"
)

func functionMatches(expectedFun any, actualFun any) bool {
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

// New tests for the programmatic TLS constructor and shared parameter parsing.
// These are behaviour-focused tests that avoid depending on a real syslog
// server.

// TestNewWithTLSConfigRequiresTLSConfigForTCPTLS ensures that using
// NewWithTLSConfig with a tcp+tls address requires a non-nil *tls.Config.
func TestNewWithTLSConfigRequiresTLSConfigForTCPTLS(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"syslog-address":  "tcp+tls://1.2.3.4:6514",
			"syslog-facility": "daemon",
			"syslog-format":   "rfc3164",
		},
		// DefaultTemplate for log tags is "{{.ID}}", which uses Info.ID() and
		// expects a non-empty ContainerID. Provide a minimal 12-character ID so
		// loggerutils.ParseLogTag succeeds without panicking.
		ContainerID:   "123456789012",
		ContainerName: "test-container",
	}

	logger, err := NewWithTLSConfig(info, nil)
	if err == nil {
		if logger != nil {
			_ = logger.Close()
		}
		t.Fatal("expected error when tls config is nil for tcp+tls syslog")
	}

	const expected = "tls config is required for tcp+tls syslog"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

// TestBuildSyslogParamsBasicUDPConfig validates that the shared parameter
// builder used by both New and NewWithTLSConfig can successfully parse a
// straightforward non-TLS configuration.
func TestBuildSyslogParamsBasicUDPConfig(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"syslog-address":  "udp://1.2.3.4:514",
			"syslog-facility": "daemon",
			"syslog-format":   "rfc3164",
		},
		ContainerID:   "123456789012",
		ContainerName: "test-container",
	}

	if _, err := buildSyslogParams(info); err != nil {
		t.Fatalf("buildSyslogParams returned error: %v", err)
	}

	// secure rfc5424 config should select the secure proto, correct address,
	// LOG_DAEMON facility, and the RFC5424 formatter/framer pair for RFC5425.
	info = logger.Info{
		Config: map[string]string{
			"syslog-address":  "tcp+tls://1.2.3.4:6514",
			"syslog-facility": "daemon",
			"syslog-format":   "rfc5424",
		},
		ContainerID:   "123456789012",
		ContainerName: "test-container",
	}

	params, err := buildSyslogParams(info)
	if err != nil {
		t.Fatalf("buildSyslogParams returned error: %v", err)
	}
	if params.proto != secureProto {
		t.Fatalf("expected proto %q, got %q", secureProto, params.proto)
	}
	if params.address != "1.2.3.4:6514" {
		t.Fatalf("expected address %q, got %q", "1.2.3.4:6514", params.address)
	}
	if params.facility != syslog.LOG_DAEMON {
		t.Fatalf("expected LOG_DAEMON facility, got %v", params.facility)
	}
	if !functionMatches(rfc5424formatterWithAppNameAsTag, params.formatter) {
		t.Fatalf("unexpected formatter for secure rfc5424 config: %#v", params.formatter)
	}
	if !functionMatches(syslog.RFC5425MessageLengthFramer, params.framer) {
		t.Fatalf("unexpected framer for secure rfc5424 config: %#v", params.framer)
	}
}

// TestNewWithTLSConfigNonSecureProto verifies that NewWithTLSConfig behaves like
// New for non-tcp+tls protocols and does not require a TLS config.
func TestNewWithTLSConfigNonSecureProto(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"syslog-address":  "udp://127.0.0.1:514",
			"syslog-facility": "daemon",
			"syslog-format":   "rfc3164",
		},
		ContainerID:   "123456789012",
		ContainerName: "test-container",
	}

	logger, err := NewWithTLSConfig(info, nil)
	if err != nil {
		t.Fatalf("expected nil error for non-secure proto, got %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger for non-secure proto")
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}
}
