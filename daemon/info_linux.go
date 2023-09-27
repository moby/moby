//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/log"
	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/rootless"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/pkg/errors"
	rkclient "github.com/rootless-containers/rootlesskit/pkg/api/client"
)

// fillPlatformInfo fills the platform related info.
func (daemon *Daemon) fillPlatformInfo(v *system.Info, sysInfo *sysinfo.SysInfo, cfg *configStore) {
	v.CgroupDriver = cgroupDriver(&cfg.Config)
	v.CgroupVersion = "1"
	if sysInfo.CgroupUnified {
		v.CgroupVersion = "2"
	}

	if v.CgroupDriver != cgroupNoneDriver {
		v.MemoryLimit = sysInfo.MemoryLimit
		v.SwapLimit = sysInfo.SwapLimit
		v.KernelMemory = sysInfo.KernelMemory
		v.KernelMemoryTCP = sysInfo.KernelMemoryTCP
		v.OomKillDisable = sysInfo.OomKillDisable
		v.CPUCfsPeriod = sysInfo.CPUCfs
		v.CPUCfsQuota = sysInfo.CPUCfs
		v.CPUShares = sysInfo.CPUShares
		v.CPUSet = sysInfo.Cpuset
		v.PidsLimit = sysInfo.PidsLimit
	}
	v.Runtimes = make(map[string]system.Runtime)
	for n, p := range stockRuntimes() {
		v.Runtimes[n] = system.Runtime{Path: p}
	}
	for n, r := range cfg.Config.Runtimes {
		v.Runtimes[n] = system.Runtime{
			Path: r.Path,
			Args: append([]string(nil), r.Args...),
		}
	}
	v.DefaultRuntime = cfg.Runtimes.Default
	v.RuncCommit.ID = "N/A"
	v.ContainerdCommit.ID = "N/A"
	v.InitCommit.ID = "N/A"

	if _, _, commit, err := parseDefaultRuntimeVersion(&cfg.Runtimes); err != nil {
		log.G(context.TODO()).Warnf(err.Error())
	} else {
		v.RuncCommit.ID = commit
	}

	if rv, err := daemon.containerd.Version(context.Background()); err == nil {
		v.ContainerdCommit.ID = rv.Revision
	} else {
		log.G(context.TODO()).Warnf("failed to retrieve containerd version: %v", err)
	}

	v.InitBinary = cfg.GetInitPath()
	if initBinary, err := cfg.LookupInitPath(); err != nil {
		log.G(context.TODO()).Warnf("failed to find docker-init: %s", err)
	} else if rv, err := exec.Command(initBinary, "--version").Output(); err == nil {
		if _, commit, err := parseInitVersion(string(rv)); err != nil {
			log.G(context.TODO()).Warnf("failed to parse %s version: %s", initBinary, err)
		} else {
			v.InitCommit.ID = commit
		}
	} else {
		log.G(context.TODO()).Warnf("failed to retrieve %s version: %s", initBinary, err)
	}

	// Set expected and actual commits to the same value to prevent the client
	// showing that the version does not match the "expected" version/commit.
	v.RuncCommit.Expected = v.RuncCommit.ID
	v.ContainerdCommit.Expected = v.ContainerdCommit.ID
	v.InitCommit.Expected = v.InitCommit.ID

	if v.CgroupDriver == cgroupNoneDriver {
		if v.CgroupVersion == "2" {
			v.Warnings = append(v.Warnings, "WARNING: Running in rootless-mode without cgroups. Systemd is required to enable cgroups in rootless-mode.")
		} else {
			v.Warnings = append(v.Warnings, "WARNING: Running in rootless-mode without cgroups. To enable cgroups in rootless-mode, you need to boot the system in cgroup v2 mode.")
		}
	} else {
		if !v.MemoryLimit {
			v.Warnings = append(v.Warnings, "WARNING: No memory limit support")
		}
		if !v.SwapLimit {
			v.Warnings = append(v.Warnings, "WARNING: No swap limit support")
		}
		if !v.KernelMemoryTCP && v.CgroupVersion == "1" {
			// kernel memory is not available for cgroup v2.
			// Warning is not printed on cgroup v2, because there is no action user can take.
			v.Warnings = append(v.Warnings, "WARNING: No kernel memory TCP limit support")
		}
		if !v.OomKillDisable && v.CgroupVersion == "1" {
			// oom kill disable is not available for cgroup v2.
			// Warning is not printed on cgroup v2, because there is no action user can take.
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
		// TODO add fields for these options in types.Info
		if !sysInfo.BlkioWeight && v.CgroupVersion == "2" {
			// blkio weight is not available on cgroup v1 since kernel 5.0.
			// Warning is not printed on cgroup v1, because there is no action user can take.
			// On cgroup v2, blkio weight is implemented using io.weight
			v.Warnings = append(v.Warnings, "WARNING: No io.weight support")
		}
		if !sysInfo.BlkioWeightDevice && v.CgroupVersion == "2" {
			v.Warnings = append(v.Warnings, "WARNING: No io.weight (per device) support")
		}
		if !sysInfo.BlkioReadBpsDevice {
			if v.CgroupVersion == "2" {
				v.Warnings = append(v.Warnings, "WARNING: No io.max (rbps) support")
			} else {
				v.Warnings = append(v.Warnings, "WARNING: No blkio throttle.read_bps_device support")
			}
		}
		if !sysInfo.BlkioWriteBpsDevice {
			if v.CgroupVersion == "2" {
				v.Warnings = append(v.Warnings, "WARNING: No io.max (wbps) support")
			} else {
				v.Warnings = append(v.Warnings, "WARNING: No blkio throttle.write_bps_device support")
			}
		}
		if !sysInfo.BlkioReadIOpsDevice {
			if v.CgroupVersion == "2" {
				v.Warnings = append(v.Warnings, "WARNING: No io.max (riops) support")
			} else {
				v.Warnings = append(v.Warnings, "WARNING: No blkio throttle.read_iops_device support")
			}
		}
		if !sysInfo.BlkioWriteIOpsDevice {
			if v.CgroupVersion == "2" {
				v.Warnings = append(v.Warnings, "WARNING: No io.max (wiops) support")
			} else {
				v.Warnings = append(v.Warnings, "WARNING: No blkio throttle.write_iops_device support")
			}
		}
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

func (daemon *Daemon) fillPlatformVersion(v *types.Version, cfg *configStore) {
	if rv, err := daemon.containerd.Version(context.Background()); err == nil {
		v.Components = append(v.Components, types.ComponentVersion{
			Name:    "containerd",
			Version: rv.Version,
			Details: map[string]string{
				"GitCommit": rv.Revision,
			},
		})
	}

	if _, ver, commit, err := parseDefaultRuntimeVersion(&cfg.Runtimes); err != nil {
		log.G(context.TODO()).Warnf(err.Error())
	} else {
		v.Components = append(v.Components, types.ComponentVersion{
			Name:    cfg.Runtimes.Default,
			Version: ver,
			Details: map[string]string{
				"GitCommit": commit,
			},
		})
	}

	if initBinary, err := cfg.LookupInitPath(); err != nil {
		log.G(context.TODO()).Warnf("failed to find docker-init: %s", err)
	} else if rv, err := exec.Command(initBinary, "--version").Output(); err == nil {
		if ver, commit, err := parseInitVersion(string(rv)); err != nil {
			log.G(context.TODO()).Warnf("failed to parse %s version: %s", initBinary, err)
		} else {
			v.Components = append(v.Components, types.ComponentVersion{
				Name:    filepath.Base(initBinary),
				Version: ver,
				Details: map[string]string{
					"GitCommit": commit,
				},
			})
		}
	} else {
		log.G(context.TODO()).Warnf("failed to retrieve %s version: %s", initBinary, err)
	}

	daemon.fillRootlessVersion(v)
}

func (daemon *Daemon) fillRootlessVersion(v *types.Version) {
	if !rootless.RunningWithRootlessKit() {
		return
	}
	rlc, err := getRootlessKitClient()
	if err != nil {
		log.G(context.TODO()).Warnf("failed to create RootlessKit client: %v", err)
		return
	}
	rlInfo, err := rlc.Info(context.TODO())
	if err != nil {
		log.G(context.TODO()).Warnf("failed to retrieve RootlessKit version: %v", err)
		return
	}
	v.Components = append(v.Components, types.ComponentVersion{
		Name:    "rootlesskit",
		Version: rlInfo.Version,
		Details: map[string]string{
			"ApiVersion":    rlInfo.APIVersion,
			"StateDir":      rlInfo.StateDir,
			"NetworkDriver": rlInfo.NetworkDriver.Driver,
			"PortDriver":    rlInfo.PortDriver.Driver,
		},
	})

	switch rlInfo.NetworkDriver.Driver {
	case "slirp4netns":
		if rv, err := exec.Command("slirp4netns", "--version").Output(); err == nil {
			if _, ver, commit, err := parseRuntimeVersion(string(rv)); err != nil {
				log.G(context.TODO()).Warnf("failed to parse slirp4netns version: %v", err)
			} else {
				v.Components = append(v.Components, types.ComponentVersion{
					Name:    "slirp4netns",
					Version: ver,
					Details: map[string]string{
						"GitCommit": commit,
					},
				})
			}
		} else {
			log.G(context.TODO()).Warnf("failed to retrieve slirp4netns version: %v", err)
		}
	case "vpnkit":
		if rv, err := exec.Command("vpnkit", "--version").Output(); err == nil {
			v.Components = append(v.Components, types.ComponentVersion{
				Name:    "vpnkit",
				Version: strings.TrimSpace(string(rv)),
			})
		} else {
			log.G(context.TODO()).Warnf("failed to retrieve vpnkit version: %v", err)
		}
	}
}

// getRootlessKitClient returns RootlessKit client
func getRootlessKitClient() (rkclient.Client, error) {
	stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR")
	if stateDir == "" {
		return nil, errors.New("environment variable `ROOTLESSKIT_STATE_DIR` is not set")
	}
	apiSock := filepath.Join(stateDir, "api.sock")
	return rkclient.New(apiSock)
}

func fillDriverWarnings(v *system.Info) {
	for _, pair := range v.DriverStatus {
		if pair[0] == "Extended file attributes" && pair[1] == "best-effort" {
			msg := fmt.Sprintf("WARNING: %s: extended file attributes from container images "+
				"will be silently discarded if the backing filesystem does not support them.\n"+
				"         CONTAINERS MAY MALFUNCTION IF EXTENDED ATTRIBUTES ARE MISSING.\n"+
				"         This is an UNSUPPORTABLE configuration for which no bug reports will be accepted.\n", v.Driver)

			v.Warnings = append(v.Warnings, msg)
			continue
		}
	}
}

// parseInitVersion parses a Tini version string, and extracts the "version"
// and "git commit" from the output.
//
// Output example from `docker-init --version`:
//
//	tini version 0.18.0 - git.fec3683
func parseInitVersion(v string) (version string, commit string, err error) {
	parts := strings.Split(v, " - ")

	if len(parts) >= 2 {
		gitParts := strings.Split(strings.TrimSpace(parts[1]), ".")
		if len(gitParts) == 2 && gitParts[0] == "git" {
			commit = gitParts[1]
		}
	}
	parts[0] = strings.TrimSpace(parts[0])
	if strings.HasPrefix(parts[0], "tini version ") {
		version = strings.TrimPrefix(parts[0], "tini version ")
	}
	if version == "" && commit == "" {
		err = errors.Errorf("unknown output format: %s", v)
	}
	return version, commit, err
}

// parseRuntimeVersion parses the output of `[runtime] --version` and extracts the
// "name", "version" and "git commit" from the output.
//
// Output example from `runc --version`:
//
//	runc version 1.0.0-rc5+dev
//	commit: 69663f0bd4b60df09991c08812a60108003fa340
//	spec: 1.0.0
func parseRuntimeVersion(v string) (runtime, version, commit string, err error) {
	lines := strings.Split(strings.TrimSpace(v), "\n")
	for _, line := range lines {
		if strings.Contains(line, "version") {
			s := strings.Split(line, "version")
			runtime = strings.TrimSpace(s[0])
			version = strings.TrimSpace(s[len(s)-1])
			continue
		}
		if strings.HasPrefix(line, "commit:") {
			commit = strings.TrimSpace(strings.TrimPrefix(line, "commit:"))
			continue
		}
	}
	if version == "" && commit == "" {
		err = errors.Errorf("unknown output format: %s", v)
	}
	return runtime, version, commit, err
}

func parseDefaultRuntimeVersion(rts *runtimes) (runtime, version, commit string, err error) {
	shim, opts, err := rts.Get(rts.Default)
	if err != nil {
		return "", "", "", err
	}
	shimopts, ok := opts.(*v2runcoptions.Options)
	if !ok {
		return "", "", "", fmt.Errorf("%s: retrieving version not supported", shim)
	}
	rt := shimopts.BinaryName
	if rt == "" {
		rt = defaultRuntimeName
	}
	rv, err := exec.Command(rt, "--version").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to retrieve %s version: %w", rt, err)
	}
	runtime, version, commit, err = parseRuntimeVersion(string(rv))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse %s version: %w", rt, err)
	}
	return runtime, version, commit, err
}

func cgroupNamespacesEnabled(sysInfo *sysinfo.SysInfo, cfg *config.Config) bool {
	return sysInfo.CgroupNamespaces && containertypes.CgroupnsMode(cfg.CgroupNamespaceMode).IsPrivate()
}

// Rootless returns true if daemon is running in rootless mode
func Rootless(cfg *config.Config) bool {
	return cfg.Rootless
}

func noNewPrivileges(cfg *config.Config) bool {
	return cfg.NoNewPrivileges
}
