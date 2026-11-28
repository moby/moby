package container

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestExecConsoleSize(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.42"), "requires API v1.42")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"))

	result, err := container.Exec(ctx, apiClient, cID, []string{"stty", "size"},
		func(ec *client.ExecCreateOptions) {
			ec.TTY = true
			ec.ConsoleSize = client.ConsoleSize{
				Height: 57,
				Width:  123,
			}
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(result.Stdout()), "57 123")
}

func TestFailedExecExitCode(t *testing.T) {
	testCases := []struct {
		doc              string
		command          []string
		tty              bool
		expectedExitCode int
		expectedError    string
	}{
		{
			doc:              "executable not found",
			command:          []string{"nonexistent"},
			expectedExitCode: 127,
			expectedError:    "executable file not found",
		},
		{
			doc:              "executable not found with TTY",
			command:          []string{"nonexistent"},
			tty:              true,
			expectedExitCode: 127,
			expectedError:    "executable file not found",
		},
		{
			doc:              "executable cannot be invoked",
			command:          []string{"/etc"},
			expectedExitCode: 126,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			ctx := setupTest(t)
			apiClient := testEnv.APIClient()

			cID := container.Run(ctx, t, apiClient)

			res, err := apiClient.ExecCreate(ctx, cID, client.ExecCreateOptions{
				AttachStdout: true,
				AttachStderr: true,
				TTY:          tc.tty,
				Cmd:          tc.command,
			})
			assert.NilError(t, err)

			resp, err := apiClient.ExecAttach(ctx, res.ID, client.ExecAttachOptions{TTY: tc.tty})
			assert.NilError(t, err)
			defer resp.Close()

			// The start error is reported on stderr; a TTY only has a single stream.
			var stdout, stderr bytes.Buffer
			if tc.tty {
				_, err = io.Copy(&stdout, resp.Reader)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(stdout.String(), tc.expectedError))
			} else {
				_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(stdout.String(), ""))
				assert.Check(t, is.Contains(stderr.String(), tc.expectedError))
			}

			inspect, err := apiClient.ExecInspect(ctx, res.ID, client.ExecInspectOptions{})
			assert.NilError(t, err)
			assert.Equal(t, inspect.ExitCode, tc.expectedExitCode)
		})
	}
}

func TestContainerExecDefaultUmask(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	users := []string{"", "1000", "nobody"}
	prettyUser := func(s string) string {
		if s == "" {
			return "unspecified"
		}
		return s
	}

	for _, runUser := range users {
		for _, execUser := range users {
			t.Run(fmt.Sprintf("user_%s_%s", prettyUser(runUser), prettyUser(execUser)), func(t *testing.T) {
				cID := container.Run(ctx, t, apiClient, container.WithUser(runUser))
				result, err := container.Exec(ctx, apiClient, cID, []string{"sh", "-c", "umask"}, func(ec *client.ExecCreateOptions) {
					ec.User = execUser
				})
				assert.NilError(t, err)
				assert.Equal(t, "0022", strings.TrimSpace(result.Stdout()))
			})
		}
	}
}

func TestContainerExecWithUmask(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.56"), "requires API v1.56")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	users := []string{"", "1000", "nobody"}
	prettyUser := func(s string) string {
		if s == "" {
			return "unspecified"
		}
		return s
	}

	tests := []struct {
		umask    uint32
		expected string
	}{
		{
			umask:    0o000,
			expected: "0000",
		},
		{
			umask:    0o777,
			expected: "0777",
		},
	}
	prettyUmask := func(u uint32) string {
		return fmt.Sprintf("%04o", u)
	}

	for _, runUser := range users {
		for _, execUser := range users {
			for _, tc := range tests {
				t.Run(fmt.Sprintf("user_%s_%s_umask_%s", prettyUser(runUser), prettyUser(execUser), prettyUmask(tc.umask)), func(t *testing.T) {
					cID := container.Run(ctx, t, apiClient, container.WithUser(runUser), func(c *container.TestContainerConfig) {
						c.HostConfig.Umask = &tc.umask
					})
					result, err := container.Exec(ctx, apiClient, cID, []string{"sh", "-c", "umask"}, func(ec *client.ExecCreateOptions) {
						ec.User = execUser
					})
					assert.NilError(t, err)
					assert.Equal(t, tc.expected, strings.TrimSpace(result.Stdout()))
				})
			}
		}
	}
}
