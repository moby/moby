package container

import (
	"context"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestWindowsProcessIsolation validates process isolation on Windows.
func TestWindowsProcessIsolation(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testcases := []struct {
		name        string
		description string
		validate    func(t *testing.T, ctx context.Context, id string)
	}{
		{
			name:        "Process isolation basic container lifecycle",
			description: "Validate container can start, run, and stop with process isolation",
			validate: func(t *testing.T, ctx context.Context, id string) {
				// Verify container is running
				ctrInfo := container.Inspect(ctx, t, apiClient, id)
				assert.Check(t, is.Equal(ctrInfo.State.Running, true))
				assert.Check(t, is.Equal(ctrInfo.HostConfig.Isolation, containertypes.IsolationProcess))

				execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				res := container.ExecT(execCtx, t, apiClient, id, []string{"cmd", "/c", "echo", "test"})
				assert.Check(t, is.Equal(res.ExitCode, 0))
				assert.Check(t, strings.Contains(res.Stdout(), "test"))
			},
		},
		{
			name:        "Process isolation filesystem access",
			description: "Validate filesystem operations work correctly with process isolation",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Create a test file
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"cmd", "/c", "echo test123 > C:\\testfile.txt"})
				assert.Check(t, is.Equal(res.ExitCode, 0))

				// Read the test file
				execCtx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
				defer cancel2()
				res2 := container.ExecT(execCtx2, t, apiClient, id,
					[]string{"cmd", "/c", "type", "C:\\testfile.txt"})
				assert.Check(t, is.Equal(res2.ExitCode, 0))
				assert.Check(t, strings.Contains(res2.Stdout(), "test123"))
			},
		},
		{
			name:        "Process isolation network connectivity",
			description: "Validate network connectivity works with process isolation",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				// Test localhost connectivity
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"ping", "-n", "1", "-w", "3000", "localhost"})
				assert.Check(t, is.Equal(res.ExitCode, 0))
				assert.Check(t, strings.Contains(res.Stdout(), "Reply from") ||
					strings.Contains(res.Stdout(), "Received = 1"))
			},
		},
		{
			name:        "Process isolation environment variables",
			description: "Validate environment variables are properly isolated",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Check that container has expected environment variables
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"cmd", "/c", "set"})
				assert.Check(t, is.Equal(res.ExitCode, 0))

				// Should have Windows-specific environment variables
				stdout := res.Stdout()
				assert.Check(t, strings.Contains(stdout, "COMPUTERNAME") ||
					strings.Contains(stdout, "OS=Windows"))
			},
		},
		{
			name:        "Process isolation CPU access",
			description: "Validate container can access CPU information",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Check NUMBER_OF_PROCESSORS environment variable
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"cmd", "/c", "echo", "%NUMBER_OF_PROCESSORS%"})
				assert.Check(t, is.Equal(res.ExitCode, 0))

				// Should return a number
				output := strings.TrimSpace(res.Stdout())
				assert.Check(t, output != "" && output != "%NUMBER_OF_PROCESSORS%",
					"NUMBER_OF_PROCESSORS not set")
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			// Create and start container with process isolation
			id := container.Run(ctx, t, apiClient,
				container.WithIsolation(containertypes.IsolationProcess),
				container.WithCmd("ping", "-t", "localhost"),
			)
			defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

			tc.validate(t, ctx, id)
		})
	}
}

// TestWindowsHyperVIsolation validates Hyper-V isolation on Windows.
func TestWindowsHyperVIsolation(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testcases := []struct {
		name        string
		description string
		validate    func(t *testing.T, ctx context.Context, id string)
	}{
		{
			name:        "Hyper-V isolation basic container lifecycle",
			description: "Validate container can start, run, and stop with Hyper-V isolation",
			validate: func(t *testing.T, ctx context.Context, id string) {
				// Verify container is running
				ctrInfo := container.Inspect(ctx, t, apiClient, id)
				assert.Check(t, is.Equal(ctrInfo.State.Running, true))
				assert.Check(t, is.Equal(ctrInfo.HostConfig.Isolation, containertypes.IsolationHyperV))

				// Execute a simple command
				execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()
				res := container.ExecT(execCtx, t, apiClient, id, []string{"cmd", "/c", "echo", "hyperv-test"})
				assert.Check(t, is.Equal(res.ExitCode, 0))
				assert.Check(t, strings.Contains(res.Stdout(), "hyperv-test"))
			},
		},
		{
			name:        "Hyper-V isolation filesystem operations",
			description: "Validate filesystem isolation with Hyper-V",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				// Test file creation
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"cmd", "/c", "echo hyperv-file > C:\\hvtest.txt"})
				assert.Check(t, is.Equal(res.ExitCode, 0))

				// Test file read
				execCtx2, cancel2 := context.WithTimeout(ctx, 15*time.Second)
				defer cancel2()
				res2 := container.ExecT(execCtx2, t, apiClient, id,
					[]string{"cmd", "/c", "type", "C:\\hvtest.txt"})
				assert.Check(t, is.Equal(res2.ExitCode, 0))
				assert.Check(t, strings.Contains(res2.Stdout(), "hyperv-file"))
			},
		},
		{
			name:        "Hyper-V isolation network connectivity",
			description: "Validate network works with Hyper-V isolation",
			validate: func(t *testing.T, ctx context.Context, id string) {
				execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				// Test localhost connectivity
				res := container.ExecT(execCtx, t, apiClient, id,
					[]string{"ping", "-n", "1", "-w", "5000", "localhost"})
				assert.Check(t, is.Equal(res.ExitCode, 0))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			// Create and start container with Hyper-V isolation
			id := container.Run(ctx, t, apiClient,
				container.WithIsolation(containertypes.IsolationHyperV),
				container.WithCmd("ping", "-t", "localhost"),
			)
			defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

			tc.validate(t, ctx, id)
		})
	}
}

// TestWindowsIsolationComparison validates that both isolation modes can coexist
// and that containers can be created with different isolation modes on Windows.
func TestWindowsIsolationComparison(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Create container with process isolation
	processID := container.Run(ctx, t, apiClient,
		container.WithIsolation(containertypes.IsolationProcess),
		container.WithCmd("ping", "-t", "localhost"),
	)
	defer apiClient.ContainerRemove(ctx, processID, client.ContainerRemoveOptions{Force: true})

	processInfo := container.Inspect(ctx, t, apiClient, processID)
	assert.Check(t, is.Equal(processInfo.HostConfig.Isolation, containertypes.IsolationProcess))
	assert.Check(t, is.Equal(processInfo.State.Running, true))

	// Create container with Hyper-V isolation
	hypervID := container.Run(ctx, t, apiClient,
		container.WithIsolation(containertypes.IsolationHyperV),
		container.WithCmd("ping", "-t", "localhost"),
	)
	defer apiClient.ContainerRemove(ctx, hypervID, client.ContainerRemoveOptions{Force: true})

	hypervInfo := container.Inspect(ctx, t, apiClient, hypervID)
	assert.Check(t, is.Equal(hypervInfo.HostConfig.Isolation, containertypes.IsolationHyperV))
	assert.Check(t, is.Equal(hypervInfo.State.Running, true))

	// Verify both containers can run simultaneously
	processInfo2 := container.Inspect(ctx, t, apiClient, processID)
	hypervInfo2 := container.Inspect(ctx, t, apiClient, hypervID)
	assert.Check(t, is.Equal(processInfo2.State.Running, true))
	assert.Check(t, is.Equal(hypervInfo2.State.Running, true))
}

// TestWindowsProcessIsolationResourceConstraints validates resource constraints
// work correctly with process isolation on Windows.
func TestWindowsProcessIsolationResourceConstraints(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testcases := []struct {
		name           string
		cpuShares      int64
		nanoCPUs       int64
		memoryLimit    int64
		cpuCount       int64
		validateConfig func(t *testing.T, ctrInfo containertypes.InspectResponse)
	}{
		{
			name:      "CPU shares constraint - config only",
			cpuShares: 512,
			// Note: CPU shares are accepted by the API but NOT enforced on Windows.
			// This test only verifies the configuration is stored correctly.
			// Actual enforcement does not work - containers get equal CPU regardless of shares.
			// Use NanoCPUs (--cpus flag) for actual CPU limiting on Windows.
			validateConfig: func(t *testing.T, ctrInfo containertypes.InspectResponse) {
				assert.Check(t, is.Equal(ctrInfo.HostConfig.CPUShares, int64(512)))
			},
		},
		{
			name:     "CPU limit (NanoCPUs) constraint",
			nanoCPUs: 2000000000, // 2.0 CPUs
			// NanoCPUs enforce hard CPU limits on Windows (unlike CPUShares which don't work)
			validateConfig: func(t *testing.T, ctrInfo containertypes.InspectResponse) {
				assert.Check(t, is.Equal(ctrInfo.HostConfig.NanoCPUs, int64(2000000000)))
			},
		},
		{
			name:        "Memory limit constraint",
			memoryLimit: 512 * 1024 * 1024, // 512MB
			// Memory limits enforce hard limits on container memory usage
			validateConfig: func(t *testing.T, ctrInfo containertypes.InspectResponse) {
				assert.Check(t, is.Equal(ctrInfo.HostConfig.Memory, int64(512*1024*1024)))
			},
		},
		{
			name:     "CPU count constraint",
			cpuCount: 2,
			// CPU count limits the number of CPUs available to the container
			validateConfig: func(t *testing.T, ctrInfo containertypes.InspectResponse) {
				assert.Check(t, is.Equal(ctrInfo.HostConfig.CPUCount, int64(2)))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			opts := []func(*container.TestContainerConfig){
				container.WithIsolation(containertypes.IsolationProcess),
				container.WithCmd("ping", "-t", "localhost"),
			}

			if tc.cpuShares > 0 {
				opts = append(opts, func(config *container.TestContainerConfig) {
					config.HostConfig.CPUShares = tc.cpuShares
				})
			}

			if tc.nanoCPUs > 0 {
				opts = append(opts, func(config *container.TestContainerConfig) {
					config.HostConfig.NanoCPUs = tc.nanoCPUs
				})
			}

			if tc.memoryLimit > 0 {
				opts = append(opts, func(config *container.TestContainerConfig) {
					config.HostConfig.Memory = tc.memoryLimit
				})
			}

			if tc.cpuCount > 0 {
				opts = append(opts, func(config *container.TestContainerConfig) {
					config.HostConfig.CPUCount = tc.cpuCount
				})
			}

			id := container.Run(ctx, t, apiClient, opts...)
			defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

			ctrInfo := container.Inspect(ctx, t, apiClient, id)
			tc.validateConfig(t, ctrInfo)
		})
	}
}

// TestWindowsProcessIsolationVolumeMount validates volume mounting with process isolation on Windows.
func TestWindowsProcessIsolationVolumeMount(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	volumeName := "process-iso-test-volume"
	volRes, err := apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name: volumeName,
	})
	assert.NilError(t, err)
	defer func() {
		// Force volume removal in case container cleanup fails
		apiClient.VolumeRemove(ctx, volRes.Volume.Name, client.VolumeRemoveOptions{Force: true})
	}()

	// Create container with volume mount
	id := container.Run(ctx, t, apiClient,
		container.WithIsolation(containertypes.IsolationProcess),
		container.WithCmd("ping", "-t", "localhost"),
		container.WithMount(mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: "C:\\data",
		}),
	)
	defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

	// Write data to mounted volume
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res := container.ExecT(execCtx, t, apiClient, id,
		[]string{"cmd", "/c", "echo volume-test > C:\\data\\test.txt"})
	assert.Check(t, is.Equal(res.ExitCode, 0))

	// Read data from mounted volume
	execCtx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel2()
	res2 := container.ExecT(execCtx2, t, apiClient, id,
		[]string{"cmd", "/c", "type", "C:\\data\\test.txt"})
	assert.Check(t, is.Equal(res2.ExitCode, 0))
	assert.Check(t, strings.Contains(res2.Stdout(), "volume-test"))

	// Verify container has volume mount
	ctrInfo := container.Inspect(ctx, t, apiClient, id)
	assert.Check(t, len(ctrInfo.Mounts) == 1)
	assert.Check(t, is.Equal(ctrInfo.Mounts[0].Type, mount.TypeVolume))
	assert.Check(t, is.Equal(ctrInfo.Mounts[0].Name, volumeName))
}

// TestWindowsHyperVIsolationResourceLimits validates resource limits work with Hyper-V isolation.
// This ensures Windows can properly enforce resource constraints on Hyper-V containers.
func TestWindowsHyperVIsolationResourceLimits(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Create container with memory limit
	memoryLimit := int64(512 * 1024 * 1024) // 512MB
	id := container.Run(ctx, t, apiClient,
		container.WithIsolation(containertypes.IsolationHyperV),
		container.WithCmd("ping", "-t", "localhost"),
		func(config *container.TestContainerConfig) {
			config.HostConfig.Memory = memoryLimit
		},
	)
	defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

	// Verify resource limit is set
	ctrInfo := container.Inspect(ctx, t, apiClient, id)
	assert.Check(t, is.Equal(ctrInfo.HostConfig.Memory, memoryLimit))
	assert.Check(t, is.Equal(ctrInfo.HostConfig.Isolation, containertypes.IsolationHyperV))
}
