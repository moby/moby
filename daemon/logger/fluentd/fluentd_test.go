package fluentd

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/moby/moby/v2/daemon/logger"
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

func TestValidateWriteTimeoutDuration(t *testing.T) {
	invalidDurations := []string{"-1", "1", "-1s"}
	for _, d := range invalidDurations {
		t.Run("invalid "+d, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{writeTimeoutKey: d})
			assert.ErrorContains(t, err, "invalid value for fluentd-write-timeout:")
		})
	}

	validDurations := map[string]time.Duration{
		"100ms": 100 * time.Millisecond,
		"10s":   10 * time.Second,
		"":      0,
	}
	for k, v := range validDurations {
		t.Run("valid "+k, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{writeTimeoutKey: k})
			assert.NilError(t, err)
			cfg, err := parseConfig(map[string]string{writeTimeoutKey: k})
			// This check is mostly redundant since it's checked in ValidateLogOpt as well.
			// This is here to guard against potential regressions in the future.
			assert.NilError(t, err)
			assert.Equal(t, cfg.WriteTimeout, v)
		})
	}
}

// TestWriteTimeoutIsEffective tests that writes timeout when the server is unresponsive.
// The test ensures that instead of hanging forever, the fluentd write operation returns
// an error when writes cannot be completed within the specified duration.
func TestWriteTimeoutIsEffective(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported")
	}

	// Create a temporary directory for the socket file
	tmpDir := t.TempDir()
	socketFile := filepath.Join(tmpDir, "fluent-logger-golang.sock")
	l, err := net.Listen("unix", socketFile)
	assert.NilError(t, err, "unable to create listener for socket %s", socketFile)
	defer l.Close()

	// This is to guard against potential run-away test scenario so that a future change
	// doesn't cause the tests suite to timeout. It "fluentd-write-timeout" is not set, this
	// test will be blocked on the socket write operation. The timeout guards against that.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var connectedWG sync.WaitGroup
	connectedWG.Add(1)

	// Start accepting connections.
	go func(ctx context.Context, wg *sync.WaitGroup) {
		wg.Done()

		for {
			conn, err := l.Accept()
			if err != nil {
				// Unable to process the connection. This can happen if the connection is closed.
				select {
				case <-ctx.Done():
					// If the context is canceled, there's nothing for us to do here.
					return
				default:
					t.Logf("Unable to accept connection: %v", err)
					continue
				}
			}

			// Handle an incoming connection. We're essentially blackholing this by not reading from
			// or writing to the connection.
			go func(ctx context.Context, conn net.Conn) {
				// Simulate unresponsive server: do nothing with the connection
				<-ctx.Done()
				_ = conn.Close()
			}(ctx, conn)
		}
	}(ctx, &connectedWG)

	f, err := New(logger.Info{
		ContainerName: "/test-container",
		ContainerID:   "container-abcdefghijklmnopqrstuvwxyz01234567890",
		Config: map[string]string{
			"fluentd-address": "unix://" + socketFile,
			"tag":             "{{.Name}}/{{.FullID}}",
			// Disabling async behavior with limited retries and buffer size lets
			// us test this in a more preditable manner for failures. Otherwise,
			// write errors could be silently consumed. The "fluentd-write-timeout"
			// flag should be equally effective regardless of async being enabled/disabled.
			"fluentd-async":         "false",
			"fluentd-max-retries":   "1",
			"fluentd-retry-wait":    "10ms",
			"fluentd-buffer-limit":  "1",
			"fluentd-write-timeout": "1ms",
		},
	})
	assert.NilError(t, err)
	defer f.Close()

	// Ensure that the server is ready to accept connections since we have disabled async mode
	// in fluentd options.
	connectedWG.Wait()

	// Attempt writing 1MiB worth of log data (all 0's) repeatedly. We should see a failure
	// after the 1st or the 2nd attempt depending on when the connection's write buffer gets
	// filled up.
	// If we don't set a write timeout on the connection, this will hang forever. But, because
	// we have a write timeout, we expect the `Log` method to return an error.
	data := make([]byte, 1024*1024)
	for range 10 {
		err = f.Log(&logger.Message{
			Line:      data,
			Timestamp: time.Now(),
		})
		if err != nil {
			break
		}
	}

	// Checks if the error contains the expected message. The full message is of the format:
	// "fluent#write: failed to write after %d attempts".
	assert.ErrorContains(t, err, "fluent#write: failed to write after")
}
