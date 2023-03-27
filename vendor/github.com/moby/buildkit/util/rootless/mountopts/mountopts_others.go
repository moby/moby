//go:build !linux
// +build !linux

package mountopts

import (
	"github.com/containerd/containerd/mount"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func UnprivilegedMountFlags(path string) ([]string, error) {
	return []string{}, nil
}

func FixUp(mounts []mount.Mount) ([]mount.Mount, error) {
	return mounts, nil
}

func FixUpOCI(mounts []specs.Mount) ([]specs.Mount, error) {
	return mounts, nil
}
