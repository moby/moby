package container // import "github.com/docker/docker/integration/container"

import (
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestExecConsoleSize(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.42"), "skip test from new feature")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"))

	result, err := container.Exec(ctx, apiClient, cID, []string{"stty", "size"},
		func(ec *containertypes.ExecOptions) {
			ec.Tty = true
			ec.ConsoleSize = &[2]uint{57, 123}
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(result.Stdout()), "57 123")
}

func TestFailedExecExitCode(t *testing.T) {
	testCases := []struct {
		doc              string
		command          []string
		expectedExitCode int
	}{
		{
			doc:              "executable not found",
			command:          []string{"nonexistent"},
			expectedExitCode: 127,
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

			result, err := container.Exec(ctx, apiClient, cID, tc.command)
			assert.NilError(t, err)

			assert.Equal(t, result.ExitCode, tc.expectedExitCode)
		})
	}
}
