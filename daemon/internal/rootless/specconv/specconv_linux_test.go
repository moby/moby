package specconv

import (
	"testing"

	"github.com/containerd/cgroups/v3"
	"github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestRemoveSysfs checks that the cgroup mount is retained when replacing
// the /sys mounts for rootless + host netns, so that the container can
// still see its own resource limits (e.g. /sys/fs/cgroup/pids.max).
// https://github.com/moby/moby/issues/44084
func TestRemoveSysfs(t *testing.T) {
	skip.If(t, cgroups.Mode() != cgroups.Unified, "test requires cgroup v2")

	spec := &specs.Spec{
		Mounts: []specs.Mount{
			{Destination: "/proc", Type: "proc", Source: "proc"},
			{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
			{Destination: "/sys/fs/cgroup", Type: "cgroup", Source: "cgroup", Options: []string{"ro", "nosuid", "noexec", "nodev"}},
		},
	}
	assert.NilError(t, removeSysfs(spec))
	assert.Check(t, is.DeepEqual([]specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/sys/fs/cgroup", Type: "cgroup", Source: "cgroup", Options: []string{"ro", "nosuid", "noexec", "nodev"}},
	}, spec.Mounts))
}

// TestRemoveSysfsCustomSysMount checks that a user-specified /sys mount is
// left alone.
func TestRemoveSysfsCustomSysMount(t *testing.T) {
	mounts := []specs.Mount{
		{Destination: "/sys", Type: "bind", Source: "/sys", Options: []string{"rbind", "ro"}},
	}
	spec := &specs.Spec{Mounts: mounts}
	assert.NilError(t, removeSysfs(spec))
	assert.Check(t, is.DeepEqual(mounts, spec.Mounts))
}
