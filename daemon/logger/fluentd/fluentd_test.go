package fluentd

import (
	"bufio"
	"context"
	"maps"
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

// TestReadWriteTimeoutsAreEffective tests that read and write timeout values are effective
// for fluentd.
func TestReadWriteTimeoutsAreEffective(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported")
	}

	for _, tc := range []struct {
		name              string
		cfg               map[string]string
		connectionHandler connHandler
		rwValidator       func(ctx context.Context, f logger.Logger)
	}{
		{
			// This test case tests that writes timeout when the server is unresponsive.
			// The test ensures that instead of hanging forever, the fluentd write operation
			// returns an error when writes cannot be completed within the specified duration.
			name: "write timeout",
			cfg: map[string]string{
				"fluentd-write-timeout": "1ms",
			},
			connectionHandler: blackholeConnectionHandler,
			rwValidator: func(ctx context.Context, f logger.Logger) {
				// Attempt writing 1MiB worth of log data (all 0's) repeatedly. We should see a failure
				// after the 1st or the 2nd attempt depending on when the connection's write buffer gets
				// filled up.
				// If we don't set a write timeout on the connection, this will hang forever. But, because
				// we have a write timeout, we expect the `Log` method to return an error.
				data := make([]byte, 1024*1024)
				var err error
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
			},
		},
		{
			// This test case tests that reads timeout when the server is unresponsive and unable to
			// send acks back.
			name: "read timeout",
			cfg: map[string]string{
				"fluentd-read-timeout": "1ms",
				"fluentd-request-ack":  "true",
			},
			connectionHandler: noAckConnectionHandler,
			rwValidator: func(ctx context.Context, f logger.Logger) {
				data := make([]byte, 1024*1024)
				done := make(chan error, 1)
				go func() {
					// Log will hang forever if the read timeout is not set. Hence, we invoke that
					// asynchronously so that we can timeout the test if something goes wrong.
					done <- f.Log(&logger.Message{
						Line:      data,
						Timestamp: time.Now(),
					})
				}()

				select {
				case err := <-done:
					// In an ideal world, we would expect the following error to be returned by fluentd:
					// "fluent#write: error reading message response ack"
					// (Ref: https://github.com/fluent/fluent-logger-golang/blob/6b31033c91e794274fd6b77c692412ae945d7d67/fluent/fluent.go#L686)
					// However, the write method returns a generic message back to the caller and consumes
					// the original error message without returning it to the caller.
					// (Ref: https://github.com/fluent/fluent-logger-golang/blob/6b31033c91e794274fd6b77c692412ae945d7d67/fluent/fluent.go#L597)
					assert.ErrorContains(t, err, "fluent#write: failed to write after 1 attempts")
				case <-ctx.Done():
					t.Log("Test timed out, which is unexpected")
					t.Fail()
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary directory for the socket file.
			tmpDir := t.TempDir()
			socketFile := filepath.Join(tmpDir, "fluent-logger-golang.sock")
			l, err := net.Listen("unix", socketFile)
			assert.NilError(t, err, "unable to create listener for socket %s", socketFile)
			defer l.Close()

			// This is to guard against potential run-away test scenario so that a future change
			// doesn't cause the tests suite to timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var connectedWG sync.WaitGroup
			connectedWG.Add(1)

			// Start accepting connections.
			go acceptConnection(t, ctx, l, tc.connectionHandler, &connectedWG)

			// Create a base configuration for fluentd logger, agnostic of the test.
			cfg := map[string]string{
				"fluentd-address": "unix://" + socketFile,
				"tag":             "{{.Name}}/{{.FullID}}",
				// Disabling async behavior with limited retries and buffer size lets us test this in a more
				// preditable manner for failures. The "fluentd-read-timeout" flag should be equally effective
				// regardless of async being enabled/disabled.
				"fluentd-async":        "false",
				"fluentd-max-retries":  "1",
				"fluentd-retry-wait":   "10ms",
				"fluentd-buffer-limit": "1",
			}
			// Update the config with test specific configs.
			maps.Copy(cfg, tc.cfg)

			f, err := New(logger.Info{
				ContainerName: "/test-container",
				ContainerID:   "container-abcdefghijklmnopqrstuvwxyz01234567890",
				Config:        cfg,
			})
			assert.NilError(t, err)
			defer closeLoggerWithContext(ctx, f)

			// Ensure that the server is ready to accept connections since we have disabled async mode
			// in fluentd options.
			connectedWG.Wait()

			tc.rwValidator(ctx, f)
		})
	}
}

// closeLoggerWithContext enables the caller to return early if logger's Close() method is stuck.
// Fluentd's Close() will be stuck as long as there's a pending write in progress since the mutex
// associated with the connection will be locked.
// Ref: https://github.com/fluent/fluent-logger-golang/blob/6b31033c91e794274fd6b77c692412ae945d7d67/fluent/fluent.go#L600
func closeLoggerWithContext(ctx context.Context, f logger.Logger) {
	doneClose := make(chan error, 1)
	go func() {
		doneClose <- f.Close()
	}()
	// Return if either Close() returns or if the context is done.
	select {
	case <-doneClose:
	case <-ctx.Done():
	}
}

func acceptConnection(
	t *testing.T,
	ctx context.Context,
	l net.Listener,
	handleConnection connHandler,
	wg *sync.WaitGroup,
) {
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

		// Handle an incoming connection.
		go handleConnection(ctx, conn)
	}
}

type connHandler func(ctx context.Context, conn net.Conn)

func blackholeConnectionHandler(ctx context.Context, conn net.Conn) {
	// Simulate unresponsive server: do nothing with the connection. We're essentially blackholing this
	// by not reading from or writing to the connection.
	<-ctx.Done()
	_ = conn.Close()
}

func noAckConnectionHandler(ctx context.Context, conn net.Conn) {
	// Create a buffered reader to read from the connection.
	reader := bufio.NewReader(conn)
	// Read data from the connection.
	_, err := reader.ReadString('\n')
	if err != nil {
		// If there's an error reading from the connection, it means the connection is closed.
		select {
		case <-ctx.Done():
			// If the context is canceled, there's nothing for us to do here.
			return
		default:
			return
		}
	}
	// Don't write an ack back. The fluent configuration is set to expect an ack from the server.
	<-ctx.Done()
	_ = conn.Close()
}
