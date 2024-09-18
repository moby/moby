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

func hasPrefix(p, prefixDir string) bool {
	prefixDir = filepath.Clean(prefixDir)
	if filepath.Base(prefixDir) == string(filepath.Separator) {
		return true
	}
	p = filepath.Clean(p)
	return p == prefixDir || strings.HasPrefix(p, prefixDir+string(filepath.Separator))
}

func dedupMounts(mnts []specs.Mount) []specs.Mount {
	ret := make([]specs.Mount, 0, len(mnts))
	visited := make(map[string]int)
	for _, mnt := range mnts {
		if j, ok := visited[mnt.Destination]; ok {
			ret[j] = mnt
		} else {
			visited[mnt.Destination] = len(ret)
			ret = append(ret, mnt)
		}
	}
	return ret
}
