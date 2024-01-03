package fluentd // import "github.com/docker/docker/daemon/logger/fluentd"
import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
)

func TestValidateLogOptReconnectInterval(t *testing.T) {
	invalidIntervals := []string{"-1", "1", "-1s", "99ms", "11s"}
	for _, v := range invalidIntervals {
		t.Run("invalid "+v, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{asyncReconnectIntervalKey: v})
			assert.ErrorContains(t, err, "invalid value for fluentd-async-reconnect-interval:")
		})
	}

	validIntervals := []string{"100ms", "10s"}
	for _, v := range validIntervals {
		t.Run("valid "+v, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{asyncReconnectIntervalKey: v})
			assert.NilError(t, err)
		})
	}
}

func TestValidateLogOptAddress(t *testing.T) {
	// ports to try, and their results
	validPorts := map[string]int{
		"":       defaultPort,
		":":      defaultPort,
		":123":   123,
		":65535": 65535,
	}
	// paths to try, which should result in an error
	paths := []string{"/", "/some-path"}

	tests := []struct {
		addr        string
		ports       map[string]int // combinations of addr + port -> expected port
		paths       []string       // paths to append to addr, should be an error for tcp/udp
		expected    location
		expectedErr string
	}{
		{
			addr: "",
			expected: location{
				protocol: defaultProtocol,
				host:     defaultHost,
				port:     defaultPort,
			},
		},
		{
			addr:  "192.168.1.1",
			ports: validPorts,
			paths: paths,
			expected: location{
				protocol: defaultProtocol,
				host:     "192.168.1.1",
			},
		},
		{
			addr:  "[::1]",
			ports: validPorts,
			paths: paths,
			expected: location{
				protocol: defaultProtocol,
				host:     "::1",
			},
		},
		{
			addr:  "example.com",
			ports: validPorts,
			paths: paths,
			expected: location{
				protocol: defaultProtocol,
				host:     "example.com",
			},
		},
		{
			addr:  "tcp://",
			paths: paths,
			expected: location{
				protocol: "tcp",
				host:     defaultHost,
				port:     defaultPort,
			},
		},
		{
			addr:  "tcp://example.com",
			ports: validPorts,
			paths: paths,
			expected: location{
				protocol: "tcp",
				host:     "example.com",
			},
		},
		{
			addr:  "tls://",
			paths: paths,
			expected: location{
				protocol: "tls",
				host:     defaultHost,
				port:     defaultPort,
			},
		},
		{
			addr:  "tls://example.com",
			ports: validPorts,
			paths: paths,
			expected: location{
				protocol: "tls",
				host:     "example.com",
			},
		},
		{
			addr:        "://",
			expectedErr: "missing protocol scheme",
		},
		{
			addr:        "something://",
			expectedErr: "unsupported scheme: 'something'",
		},
		{
			addr:        "udp://",
			expectedErr: "unsupported scheme: 'udp'",
		},
		{
			addr:        "unixgram://",
			expectedErr: "unsupported scheme: 'unixgram'",
		},
		{
			addr:        "tcp+tls://",
			expectedErr: "unsupported scheme: 'tcp+tls'",
		},
		{
			addr:        "corrupted:c",
			expectedErr: "invalid port",
		},
		{
			addr:        "tcp://example.com:port",
			expectedErr: "invalid port",
		},
		{
			addr:        "tcp://example.com:-1",
			expectedErr: "invalid port",
		},
		{
			addr:        "tcp://example.com:65536",
			expectedErr: "invalid port",
		},
		{
			addr:        "unix://",
			expectedErr: "path is empty",
		},
		{
			addr: "unix:///some/socket.sock",
			expected: location{
				protocol: "unix",
				path:     "/some/socket.sock",
			},
		},
		{
			addr: "unix:///some/socket.sock:80", // unusual, but technically valid
			expected: location{
				protocol: "unix",
				path:     "/some/socket.sock:80",
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		if len(tc.ports) == 0 {
			tc.ports = map[string]int{"": tc.expected.port}
		}

		// always try empty paths; add paths to try if the test specifies it
		tc.paths = append([]string{""}, tc.paths...)
		for port, expectedPort := range tc.ports {
			for _, path := range tc.paths {
				address := tc.addr + port + path
				expected := tc.expected
				expected.port = expectedPort
				t.Run(address, func(t *testing.T) {
					err := ValidateLogOpt(map[string]string{addressKey: address})
					if path != "" {
						assert.ErrorContains(t, err, "should not contain a path element")
						return
					}
					if tc.expectedErr != "" {
						assert.ErrorContains(t, err, tc.expectedErr)
						return
					}

					assert.NilError(t, err)
					addr, _ := parseAddress(address)
					assert.DeepEqual(t, expected, *addr, cmp.AllowUnexported(location{}))
				})
			}
		}
	}
}
