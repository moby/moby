//go:build !windows
// +build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultRuntimeName = "runc"

	linuxShimV2 = "io.containerd.runc.v2"
)

func configureRuntimes(conf *config.Config) {
	if conf.DefaultRuntime == "" {
		conf.DefaultRuntime = config.StockRuntimeName
	}
	if conf.Runtimes == nil {
		conf.Runtimes = make(map[string]types.Runtime)
	}
	conf.Runtimes[config.LinuxV2RuntimeName] = types.Runtime{Path: defaultRuntimeName, Shim: defaultV2ShimConfig(conf, defaultRuntimeName)}
	conf.Runtimes[config.StockRuntimeName] = conf.Runtimes[config.LinuxV2RuntimeName]
}

func defaultV2ShimConfig(conf *config.Config, runtimePath string) *types.ShimConfig {
	return &types.ShimConfig{
		Binary: linuxShimV2,
		Opts: &v2runcoptions.Options{
			BinaryName:    runtimePath,
			Root:          filepath.Join(conf.ExecRoot, "runtime-"+defaultRuntimeName),
			SystemdCgroup: UsingSystemd(conf),
			NoPivotRoot:   os.Getenv("DOCKER_RAMDISK") != "",
		},
	}
}

func (daemon *Daemon) loadRuntimes() error {
	return daemon.initRuntimes(daemon.configStore.Runtimes)
}

func (daemon *Daemon) initRuntimes(runtimes map[string]types.Runtime) (err error) {
	runtimeDir := filepath.Join(daemon.configStore.Root, "runtimes")
	// Remove old temp directory if any
	os.RemoveAll(runtimeDir + "-old")
	tmpDir, err := ioutils.TempDir(daemon.configStore.Root, "gen-runtimes")
	if err != nil {
		return errors.Wrap(err, "failed to get temp dir to generate runtime scripts")
	}
	defer func() {
		if err != nil {
			if err1 := os.RemoveAll(tmpDir); err1 != nil {
				logrus.WithError(err1).WithField("dir", tmpDir).
					Warn("failed to remove tmp dir")
			}
			return
		}

		if err = os.Rename(runtimeDir, runtimeDir+"-old"); err != nil {
			return
		}
		if err = os.Rename(tmpDir, runtimeDir); err != nil {
			err = errors.Wrap(err, "failed to setup runtimes dir, new containers may not start")
			return
		}
		if err = os.RemoveAll(runtimeDir + "-old"); err != nil {
			logrus.WithError(err).WithField("dir", tmpDir).
				Warn("failed to remove old runtimes dir")
		}
	}()

	for name, rt := range runtimes {
		if len(rt.Args) > 0 {
			script := filepath.Join(tmpDir, name)
			content := fmt.Sprintf("#!/bin/sh\n%s %s $@\n", rt.Path, strings.Join(rt.Args, " "))
			if err := os.WriteFile(script, []byte(content), 0700); err != nil {
				return err
			}
		}
		if rt.Shim == nil {
			rt.Shim = defaultV2ShimConfig(daemon.configStore, rt.Path)
		}
	}
	return nil
}

// rewriteRuntimePath is used for runtimes which have custom arguments supplied.
// This is needed because the containerd API only calls the OCI runtime binary, there is no options for extra arguments.
// To support this case, the daemon wraps the specified runtime in a script that passes through those arguments.
func (daemon *Daemon) rewriteRuntimePath(name, p string, args []string) (string, error) {
	if len(args) == 0 {
		return p, nil
	}

	// Check that the runtime path actually exists here so that we can return a well known error.
	if _, err := exec.LookPath(p); err != nil {
		return "", errors.Wrap(err, "error while looking up the specified runtime path")
	}

	return filepath.Join(daemon.configStore.Root, "runtimes", name), nil
}

func (daemon *Daemon) getRuntime(name string) (*types.Runtime, error) {
	rt := daemon.configStore.GetRuntime(name)
	if rt == nil {
		if !isPermissibleC8dRuntimeName(name) {
			return nil, errdefs.InvalidParameter(errors.Errorf("unknown or invalid runtime name: %s", name))
		}
		return &types.Runtime{Shim: &types.ShimConfig{Binary: name}}, nil
	}

	if len(rt.Args) > 0 {
		p, err := daemon.rewriteRuntimePath(name, rt.Path, rt.Args)
		if err != nil {
			return nil, err
		}
		rt.Path = p
		rt.Args = nil
	}

	if rt.Shim == nil {
		rt.Shim = defaultV2ShimConfig(daemon.configStore, rt.Path)
	}

	return rt, nil
}

// isPermissibleC8dRuntimeName tests whether name is safe to pass into
// containerd as a runtime name, and whether the name is well-formed.
// It does not check if the runtime is installed.
//
// A runtime name containing slash characters is interpreted by containerd as
// the path to a runtime binary. If we allowed this, anyone with Engine API
// access could get containerd to execute an arbitrary binary as root. Although
// Engine API access is already equivalent to root on the host, the runtime name
// has not historically been a vector to run arbitrary code as root so users are
// not expecting it to become one.
//
// This restriction is not configurable. There are viable workarounds for
// legitimate use cases: administrators and runtime developers can make runtimes
// available for use with Docker by installing them onto PATH following the
// [binary naming convention] for containerd Runtime v2.
//
// [binary naming convention]: https://github.com/containerd/containerd/blob/main/runtime/v2/README.md#binary-naming
func isPermissibleC8dRuntimeName(name string) bool {
	// containerd uses a rather permissive test to validate runtime names:
	//
	//   - Any name for which filepath.IsAbs(name) is interpreted as the absolute
	//     path to a shim binary. We want to block this behaviour.
	//   - Any name which contains at least one '.' character and no '/' characters
	//     and does not begin with a '.' character is a valid runtime name. The shim
	//     binary name is derived from the final two components of the name and
	//     searched for on the PATH. The name "a.." is technically valid per
	//     containerd's implementation: it would resolve to a binary named
	//     "containerd-shim---".
	//
	// https://github.com/containerd/containerd/blob/11ded166c15f92450958078cd13c6d87131ec563/runtime/v2/manager.go#L297-L317
	// https://github.com/containerd/containerd/blob/11ded166c15f92450958078cd13c6d87131ec563/runtime/v2/shim/util.go#L83-L93
	return !filepath.IsAbs(name) && !strings.ContainsRune(name, '/') && shim.BinaryName(name) != ""
}
