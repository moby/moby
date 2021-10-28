package oci

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func withRemovedMount(destination string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		newMounts := []specs.Mount{}
		for _, o := range s.Mounts {
			if o.Destination != destination {
				newMounts = append(newMounts, o)
			}
		}
		s.Mounts = newMounts

		return nil
	}
}

func withROBind(src, dest string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: dest,
			Type:        "bind",
			Source:      src,
			Options:     []string{"nosuid", "noexec", "nodev", "rbind", "ro"},
		})
		return nil
	}
}

func withCGroup() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"ro", "nosuid", "noexec", "nodev"},
		})
		return nil
	}

}

func hasPrefix(p, prefixDir string) bool {
	prefixDir = filepath.Clean(prefixDir)
	if filepath.Base(prefixDir) == string(filepath.Separator) {
		return true
	}
	p = filepath.Clean(p)
	return p == prefixDir || strings.HasPrefix(p, prefixDir+string(filepath.Separator))
}

func removeMountsWithPrefix(mounts []specs.Mount, prefixDir string) []specs.Mount {
	var ret []specs.Mount
	for _, m := range mounts {
		if !hasPrefix(m.Destination, prefixDir) {
			ret = append(ret, m)
		}
	}
	return ret
}

func withBoundProc() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = removeMountsWithPrefix(s.Mounts, "/proc")
		procMount := specs.Mount{
			Destination: "/proc",
			Type:        "bind",
			Source:      "/proc",
			// NOTE: "rbind"+"ro" does not make /proc read-only recursively.
			// So we keep maskedPath and readonlyPaths (although not mandatory for rootless mode)
			Options: []string{"rbind"},
		}
		s.Mounts = append([]specs.Mount{procMount}, s.Mounts...)

		var maskedPaths []string
		for _, s := range s.Linux.MaskedPaths {
			if !hasPrefix(s, "/proc") {
				maskedPaths = append(maskedPaths, s)
			}
		}
		s.Linux.MaskedPaths = maskedPaths

		var readonlyPaths []string
		for _, s := range s.Linux.ReadonlyPaths {
			if !hasPrefix(s, "/proc") {
				readonlyPaths = append(readonlyPaths, s)
			}
		}
		s.Linux.ReadonlyPaths = readonlyPaths

		return nil
	}
}

func dedupMounts(mnts []specs.Mount) []specs.Mount {
	ret := make([]specs.Mount, 0, len(mnts))
	visited := make(map[string]int)
	for i, mnt := range mnts {
		if j, ok := visited[mnt.Destination]; ok {
			ret[j] = mnt
		} else {
			visited[mnt.Destination] = i
			ret = append(ret, mnt)
		}
	}
	return ret
}
