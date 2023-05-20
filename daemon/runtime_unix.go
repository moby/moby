//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd/shimopts"
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
	conf.Runtimes[config.LinuxV2RuntimeName] = types.Runtime{Path: defaultRuntimeName, ShimConfig: defaultV2ShimConfig(conf, defaultRuntimeName)}
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
	runtimeOldDir := runtimeDir + "-old"
	// Remove old temp directory if any
	os.RemoveAll(runtimeOldDir)
	tmpDir, err := os.MkdirTemp(daemon.configStore.Root, "gen-runtimes")
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

		if err = os.Rename(runtimeDir, runtimeOldDir); err != nil {
			logrus.WithError(err).WithField("dir", runtimeDir).
				Warn("failed to rename runtimes dir to old. Will try to removing it")
			if err = os.RemoveAll(runtimeDir); err != nil {
				logrus.WithError(err).WithField("dir", runtimeDir).
					Warn("failed to remove old runtimes dir")
				return
			}
		}
		if err = os.Rename(tmpDir, runtimeDir); err != nil {
			err = errors.Wrap(err, "failed to setup runtimes dir, new containers may not start")
			return
		}
		if err = os.RemoveAll(runtimeOldDir); err != nil {
			logrus.WithError(err).WithField("dir", runtimeOldDir).
				Warn("failed to remove old runtimes dir")
		}
	}()

	for name := range runtimes {
		rt := runtimes[name]
		if rt.Path == "" && rt.Type == "" {
			return errors.Errorf("runtime %s: either a runtimeType or a path must be configured", name)
		}
		if rt.Path != "" {
			if rt.Type != "" {
				return errors.Errorf("runtime %s: cannot configure both path and runtimeType for the same runtime", name)
			}
			if len(rt.Options) > 0 {
				return errors.Errorf("runtime %s: options cannot be used with a path runtime", name)
			}

			if len(rt.Args) > 0 {
				script := filepath.Join(tmpDir, name)
				content := fmt.Sprintf("#!/bin/sh\n%s %s $@\n", rt.Path, strings.Join(rt.Args, " "))
				if err := os.WriteFile(script, []byte(content), 0700); err != nil {
					return err
				}
			}
			rt.ShimConfig = defaultV2ShimConfig(daemon.configStore, daemon.rewriteRuntimePath(name, rt.Path, rt.Args))
		} else {
			if len(rt.Args) > 0 {
				return errors.Errorf("runtime %s: args cannot be used with a runtimeType runtime", name)
			}
			// Unlike implicit runtimes, there is no restriction on configuring a shim by path.
			rt.ShimConfig = &types.ShimConfig{Binary: rt.Type}
			if len(rt.Options) > 0 {
				// It has to be a pointer type or there'll be a panic in containerd/typeurl when we try to start the container.
				rt.ShimConfig.Opts, err = shimopts.Generate(rt.Type, rt.Options)
				if err != nil {
					return errors.Wrapf(err, "runtime %v", name)
				}
			}
		}
		runtimes[name] = rt
	}
	return nil
}

// rewriteRuntimePath is used for runtimes which have custom arguments supplied.
// This is needed because the containerd API only calls the OCI runtime binary, there is no options for extra arguments.
// To support this case, the daemon wraps the specified runtime in a script that passes through those arguments.
func (daemon *Daemon) rewriteRuntimePath(name, p string, args []string) string {
	if len(args) == 0 {
		return p
	}

	return filepath.Join(daemon.configStore.Root, "runtimes", name)
}

func (daemon *Daemon) getRuntime(name string) (shim string, opts interface{}, err error) {
	rt := daemon.configStore.GetRuntime(name)
	if rt == nil {
		if !config.IsPermissibleC8dRuntimeName(name) {
			return "", nil, errdefs.InvalidParameter(errors.Errorf("unknown or invalid runtime name: %s", name))
		}
		return name, nil, nil
	}

	if len(rt.Args) > 0 {
		// Check that the path of the runtime which the script wraps actually exists so
		// that we can return a well known error which references the configured path
		// instead of the wrapper script's.
		if _, err := exec.LookPath(rt.Path); err != nil {
			return "", nil, errors.Wrap(err, "error while looking up the specified runtime path")
		}
	}

	if rt.ShimConfig == nil {
		// Should never happen as daemon.initRuntimes always sets
		// ShimConfig and config reloading is synchronized.
		err := errdefs.System(errors.Errorf("BUG: runtime %s: rt.ShimConfig == nil", name))
		logrus.Error(err)
		return "", nil, err
	}

	return rt.ShimConfig.Binary, rt.ShimConfig.Opts, nil
}
