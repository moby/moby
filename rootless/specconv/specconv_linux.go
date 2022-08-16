package specconv // import "github.com/docker/docker/rootless/specconv"

import (
	"os"
	"path"
	"strconv"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// ToRootless converts spec to be compatible with "rootless" runc.
// * Remove non-supported cgroups
// * Fix up OOMScoreAdj
// * Fix up /proc if --pid=host
//
// v2Controllers should be non-nil only if running with v2 and systemd.
func ToRootless(spec *specs.Spec, v2Controllers []string) error {
	return toRootless(spec, v2Controllers, getCurrentOOMScoreAdj())
}

func getCurrentOOMScoreAdj() int {
	b, err := os.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		logrus.WithError(err).Warn("failed to read /proc/self/oom_score_adj")
		return 0
	}
	s := string(b)
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		logrus.WithError(err).Warnf("failed to parse /proc/self/oom_score_adj (%q)", s)
		return 0
	}
	return i
}

func toRootless(spec *specs.Spec, v2Controllers []string, currentOOMScoreAdj int) error {
	if len(v2Controllers) == 0 {
		// Remove cgroup settings.
		spec.Linux.Resources = nil
		spec.Linux.CgroupsPath = ""
	} else {
		if spec.Linux.Resources != nil {
			m := make(map[string]struct{})
			for _, s := range v2Controllers {
				m[s] = struct{}{}
			}
			// Remove devices: https://github.com/containers/crun/issues/255
			spec.Linux.Resources.Devices = nil
			if _, ok := m["memory"]; !ok {
				spec.Linux.Resources.Memory = nil
			}
			if _, ok := m["cpu"]; !ok {
				spec.Linux.Resources.CPU = nil
			}
			if _, ok := m["cpuset"]; !ok {
				if spec.Linux.Resources.CPU != nil {
					spec.Linux.Resources.CPU.Cpus = ""
					spec.Linux.Resources.CPU.Mems = ""
				}
			}
			if _, ok := m["pids"]; !ok {
				spec.Linux.Resources.Pids = nil
			}
			if _, ok := m["io"]; !ok {
				spec.Linux.Resources.BlockIO = nil
			}
			if _, ok := m["rdma"]; !ok {
				spec.Linux.Resources.Rdma = nil
			}
			spec.Linux.Resources.HugepageLimits = nil
			spec.Linux.Resources.Network = nil
		}
	}

	if spec.Process.OOMScoreAdj != nil && *spec.Process.OOMScoreAdj < currentOOMScoreAdj {
		*spec.Process.OOMScoreAdj = currentOOMScoreAdj
	}

	// Fix up /proc if --pid=host
	pidHost, err := isPidHost(spec)
	if err != nil {
		return err
	}
	if !pidHost {
		return nil
	}
	return bindMountHostProcfs(spec)
}

func isPidHost(spec *specs.Spec) (bool, error) {
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.PIDNamespace {
			if ns.Path == "" {
				return false, nil
			}
			pidNS, err := os.Readlink(ns.Path)
			if err != nil {
				return false, err
			}
			selfPidNS, err := os.Readlink("/proc/self/ns/pid")
			if err != nil {
				return false, err
			}
			return pidNS == selfPidNS, nil
		}
	}
	return true, nil
}

func bindMountHostProcfs(spec *specs.Spec) error {
	// Replace procfs mount with rbind
	// https://github.com/containers/podman/blob/v3.0.0-rc1/pkg/specgen/generate/oci.go#L248-L257
	for i, m := range spec.Mounts {
		if path.Clean(m.Destination) == "/proc" {
			newM := specs.Mount{
				Destination: "/proc",
				Type:        "bind",
				Source:      "/proc",
				Options:     []string{"rbind", "nosuid", "noexec", "nodev"},
			}
			spec.Mounts[i] = newM
		}
	}

	// Remove ReadonlyPaths for /proc/*
	newROP := spec.Linux.ReadonlyPaths[:0]
	for _, s := range spec.Linux.ReadonlyPaths {
		s = path.Clean(s)
		if !strings.HasPrefix(s, "/proc/") {
			newROP = append(newROP, s)
		}
	}
	spec.Linux.ReadonlyPaths = newROP

	return nil
}
