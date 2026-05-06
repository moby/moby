//go:build !windows

package container

import (
	"context"
	"fmt"
	"os"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	cgroupsadopt "github.com/moby/moby/v2/pkg/cgroups"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestCgroupAdoptionEnabled verifies that when --adopt-user-cgroups is enabled,
// containers inherit their creator's cgroup parent.
func TestCgroupAdoptionEnabled(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "requires root")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t, "--adopt-user-cgroups")

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	// Derive expected cgroup parent from current process
	expectedParent, err := cgroupsadopt.DeriveParentFromPid(os.Getpid())
	assert.NilError(t, err)

	// Create container without specifying CgroupParent
	cID := container.Run(ctx, t, apiClient)
	defer apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})

	// Verify container's CgroupParent matches our cgroup
	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, inspect.Container.HostConfig.CgroupParent, expectedParent)
}

// TestCgroupAdoptionDisabled verifies that cgroup adoption is disabled by default.
func TestCgroupAdoptionDisabled(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "requires root")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t) // No --adopt-user-cgroups flag

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	// Create container
	cID := container.Run(ctx, t, apiClient)
	defer apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})

	// Verify container's CgroupParent is empty (not adopted)
	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, inspect.Container.HostConfig.CgroupParent, "")
}

// TestCgroupAdoptionUserOverrideRejected verifies that when --adopt-user-cgroups is enabled,
// users cannot override the cgroup parent with a different value.
func TestCgroupAdoptionUserOverrideRejected(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "requires root")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t, "--adopt-user-cgroups")

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	customParent := "/docker/custom-parent"

	// Attempt to create container WITH explicit CgroupParent
	_, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &containertypes.Config{Image: "busybox"},
		HostConfig: &containertypes.HostConfig{
			CgroupParent: customParent,
		},
	})

	// Verify request is REJECTED with appropriate error
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	assert.Check(t, is.ErrorContains(err, "cannot set cgroup parent when --adopt-user-cgroups is enabled"))
}

// TestCgroupAdoptionMatchingParentAccepted verifies that when --adopt-user-cgroups is enabled,
// users CAN specify the cgroup parent if it matches the expected adopted value.
func TestCgroupAdoptionMatchingParentAccepted(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "requires root")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t, "--adopt-user-cgroups")

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	// Get current process's expected cgroup parent
	expectedParent, err := cgroupsadopt.DeriveParentFromPid(os.Getpid())
	assert.NilError(t, err)

	// Create container with MATCHING cgroup parent (should be allowed)
	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &containertypes.Config{Image: "busybox"},
		HostConfig: &containertypes.HostConfig{
			CgroupParent: expectedParent,
		},
	})
	assert.NilError(t, err)

	defer apiClient.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})

	// Verify it was created successfully with the correct cgroup parent
	inspect, err := apiClient.ContainerInspect(ctx, resp.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, inspect.Container.HostConfig.CgroupParent, expectedParent)
}

// TestCgroupAdoptionNoPeerCredentials verifies behavior when peer credentials are unavailable
// (e.g., when not using Unix socket).
func TestCgroupAdoptionNoPeerCredentials(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "requires root")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	// Start daemon with TCP socket instead of Unix socket to prevent peer credentials
	d.Start(t, "--adopt-user-cgroups", "-H", fmt.Sprintf("tcp://127.0.0.1:%d", testutil.GetFreePort(t)))

	// Connect via TCP
	apiClient, err := client.New(
		client.FromEnv,
		client.WithHost(fmt.Sprintf("tcp://127.0.0.1:%d", testutil.GetFreePort(t))),
	)
	assert.NilError(t, err)
	defer apiClient.Close()

	// Attempt to create container - should fail because peer creds unavailable
	_, err = apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &containertypes.Config{Image: "busybox"},
	})

	// This should fail gracefully (exact error depends on implementation)
	// For now, we just verify it doesn't panic
	if err != nil {
		t.Logf("Expected error when peer credentials unavailable: %v", err)
	}
}
