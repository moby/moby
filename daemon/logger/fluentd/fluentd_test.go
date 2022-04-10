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

	// paths to try
	paths := []string{"/", "/some-path"}

	tests := []struct {
		addr        string
		paths       []string // paths to append to addr, should be an error for tcp/udp
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
			paths: paths,
			expected: location{
				port:     defaultPort,
				protocol: defaultProtocol,
			},
		},
		{
			addr:  "[::1]",
			paths: paths,
			expected: location{
				port:     defaultPort,
				protocol: defaultProtocol,
			},
		},
		{
			addr:  "example.com",
			paths: paths,
			expected: location{
				port:     defaultPort,
				protocol: defaultProtocol,
			},
		},
		{
			addr:  "tcp://",
			paths: paths,
			expected: location{
				protocol: "tcp",
				port:     defaultPort,
			},
		},
		{
			addr:  "tcp://example.com",
			paths: paths,
			expected: location{
				protocol: "tcp",
				port:     defaultPort,
			},
		},
		{
			addr:  "tcp://example.com:65535",
			paths: paths,
			expected: location{
				protocol: "tcp",
				host:     "example.com",
				port:     65535,
			},
		},
		{
			addr:        "://",
			expectedErr: "invalid syntax",
		},
		{
			addr:        "something://",
			expectedErr: "invalid syntax",
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
		if len(tc.paths) == 0 {
			tc.paths = []string{""}
		}
		for _, path := range tc.paths {
			address := tc.addr + path
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
				assert.DeepEqual(t, tc.expected, *addr, cmp.AllowUnexported(location{}))
			})
		}
	}
}
