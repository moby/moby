package oci

import (
	"context"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// MountOpts sets oci spec specific info for mount points
type MountOpts func([]specs.Mount) []specs.Mount

//GetMounts returns default required for buildkit
// https://github.com/moby/buildkit/issues/429
func GetMounts(ctx context.Context, mountOpts ...MountOpts) []specs.Mount {
	mounts := []specs.Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "ro"},
		},
	}
	for _, o := range mountOpts {
		mounts = o(mounts)
	}
	return mounts
}

func withROBind(src, dest string) func(m []specs.Mount) []specs.Mount {
	return func(m []specs.Mount) []specs.Mount {
		m = append(m, specs.Mount{
			Destination: dest,
			Type:        "bind",
			Source:      src,
			Options:     []string{"rbind", "ro"},
		})
		return m
	}
}
