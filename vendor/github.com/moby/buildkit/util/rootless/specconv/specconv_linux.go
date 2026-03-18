package specconv

import (
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ToRootless converts spec to be compatible with "rootless" runc.
// * Remove /sys mount
// * Remove cgroups
//
// See docs/rootless.md for the supported runc revision.
func ToRootless(spec *specs.Spec) error {
	// Remove /sys mount because we can't mount /sys when the daemon netns
	// is not unshared from the host.
	//
	// Instead, we could bind-mount /sys from the host, however, `rbind, ro`
	// does not make /sys/fs/cgroup read-only (and we can't bind-mount /sys
	// without rbind)
	//
	// PR for making /sys/fs/cgroup read-only is proposed, but it is very
	// complicated: https://github.com/opencontainers/runc/pull/1869
	//
	// For buildkit usecase, we suppose we don't need to provide /sys to
	// containers and remove /sys mount as a workaround.
	var mounts []specs.Mount
	for _, mount := range spec.Mounts {
		if strings.HasPrefix(mount.Destination, "/sys") {
			continue
		}
		mounts = append(mounts, mount)
	}
	spec.Mounts = mounts

	// Remove cgroups so as to avoid `container_linux.go:337: starting container process caused "process_linux.go:280: applying cgroup configuration for process caused \"mkdir /sys/fs/cgroup/cpuset/buildkit: permission denied\""`
	spec.Linux.Resources = nil
	spec.Linux.CgroupsPath = ""
	return nil
}
