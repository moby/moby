// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// fillPlatformInfo fills the platform related info.
func (daemon *Daemon) fillPlatformInfo(v *types.Info, sysInfo *sysinfo.SysInfo) {
	v.MemoryLimit = sysInfo.MemoryLimit
	v.SwapLimit = sysInfo.SwapLimit
	v.KernelMemory = sysInfo.KernelMemory
	v.OomKillDisable = sysInfo.OomKillDisable
	v.CPUCfsPeriod = sysInfo.CPUCfsPeriod
	v.CPUCfsQuota = sysInfo.CPUCfsQuota
	v.CPUShares = sysInfo.CPUShares
	v.CPUSet = sysInfo.Cpuset
	v.Runtimes = daemon.configStore.GetAllRuntimes()
	v.DefaultRuntime = daemon.configStore.GetDefaultRuntimeName()
	v.InitBinary = daemon.configStore.GetInitPath()

	defaultRuntimeBinary := daemon.configStore.GetRuntime(v.DefaultRuntime).Path
	if rv, err := exec.Command(defaultRuntimeBinary, "--version").Output(); err == nil {
		parts := strings.Split(strings.TrimSpace(string(rv)), "\n")
		if len(parts) == 3 {
			parts = strings.Split(parts[1], ": ")
			if len(parts) == 2 {
				v.RuncCommit.ID = strings.TrimSpace(parts[1])
			}
		}

		if v.RuncCommit.ID == "" {
			logrus.Warnf("failed to retrieve %s version: unknown output format: %s", defaultRuntimeBinary, string(rv))
			v.RuncCommit.ID = "N/A"
		}
	} else {
		logrus.Warnf("failed to retrieve %s version: %v", defaultRuntimeBinary, err)
		v.RuncCommit.ID = "N/A"
	}

	// runc is now shipped as a separate package. Set "expected" to same value
	// as "ID" to prevent clients from reporting a version-mismatch
	v.RuncCommit.Expected = v.RuncCommit.ID

	if rv, err := daemon.containerd.Version(context.Background()); err == nil {
		v.ContainerdCommit.ID = rv.Revision
	} else {
		logrus.Warnf("failed to retrieve containerd version: %v", err)
		v.ContainerdCommit.ID = "N/A"
	}

	// containerd is now shipped as a separate package. Set "expected" to same
	// value as "ID" to prevent clients from reporting a version-mismatch
	v.ContainerdCommit.Expected = v.ContainerdCommit.ID

	defaultInitBinary := daemon.configStore.GetInitPath()
	if rv, err := exec.Command(defaultInitBinary, "--version").Output(); err == nil {
		ver, err := parseInitVersion(string(rv))

		if err != nil {
			logrus.Warnf("failed to retrieve %s version: %s", defaultInitBinary, err)
		}
		v.InitCommit = ver
	} else {
		logrus.Warnf("failed to retrieve %s version: %s", defaultInitBinary, err)
		v.InitCommit.ID = "N/A"
	}

	if !v.MemoryLimit {
		v.Warnings = append(v.Warnings, "WARNING: No memory limit support")
	}
	if !v.SwapLimit {
		v.Warnings = append(v.Warnings, "WARNING: No swap limit support")
	}
	if !v.KernelMemory {
		v.Warnings = append(v.Warnings, "WARNING: No kernel memory limit support")
	}
	if !v.OomKillDisable {
		v.Warnings = append(v.Warnings, "WARNING: No oom kill disable support")
	}
	if !v.CPUCfsQuota {
		v.Warnings = append(v.Warnings, "WARNING: No cpu cfs quota support")
	}
	if !v.CPUCfsPeriod {
		v.Warnings = append(v.Warnings, "WARNING: No cpu cfs period support")
	}
	if !v.CPUShares {
		v.Warnings = append(v.Warnings, "WARNING: No cpu shares support")
	}
	if !v.CPUSet {
		v.Warnings = append(v.Warnings, "WARNING: No cpuset support")
	}
	if !v.IPv4Forwarding {
		v.Warnings = append(v.Warnings, "WARNING: IPv4 forwarding is disabled")
	}
	if !v.BridgeNfIptables {
		v.Warnings = append(v.Warnings, "WARNING: bridge-nf-call-iptables is disabled")
	}
	if !v.BridgeNfIP6tables {
		v.Warnings = append(v.Warnings, "WARNING: bridge-nf-call-ip6tables is disabled")
	}
}

func fillDriverWarnings(v *types.Info) {
	if v.DriverStatus == nil {
		return
	}
	for _, pair := range v.DriverStatus {
		if pair[0] == "Data loop file" {
			msg := fmt.Sprintf("WARNING: %s: usage of loopback devices is "+
				"strongly discouraged for production use.\n         "+
				"Use `--storage-opt dm.thinpooldev` to specify a custom block storage device.", v.Driver)

			v.Warnings = append(v.Warnings, msg)
			continue
		}
		if pair[0] == "Supports d_type" && pair[1] == "false" {
			backingFs := getBackingFs(v)

			msg := fmt.Sprintf("WARNING: %s: the backing %s filesystem is formatted without d_type support, which leads to incorrect behavior.\n", v.Driver, backingFs)
			if backingFs == "xfs" {
				msg += "         Reformat the filesystem with ftype=1 to enable d_type support.\n"
			}
			msg += "         Running without d_type support will not be supported in future releases."

			v.Warnings = append(v.Warnings, msg)
			continue
		}
	}
}

func getBackingFs(v *types.Info) string {
	if v.DriverStatus == nil {
		return ""
	}
	for _, pair := range v.DriverStatus {
		if pair[0] == "Backing Filesystem" {
			return pair[1]
		}
	}
	return ""
}

// parseInitVersion parses a Tini version string, and extracts the version.
func parseInitVersion(v string) (types.Commit, error) {
	version := types.Commit{ID: "", Expected: dockerversion.InitCommitID}
	parts := strings.Split(strings.TrimSpace(v), " - ")

	if len(parts) >= 2 {
		gitParts := strings.Split(parts[1], ".")
		if len(gitParts) == 2 && gitParts[0] == "git" {
			version.ID = gitParts[1]
			version.Expected = dockerversion.InitCommitID[0:len(version.ID)]
		}
	}
	if version.ID == "" && strings.HasPrefix(parts[0], "tini version ") {
		version.ID = "v" + strings.TrimPrefix(parts[0], "tini version ")
	}
	if version.ID == "" {
		version.ID = "N/A"
		return version, errors.Errorf("unknown output format: %s", v)
	}
	return version, nil
}
