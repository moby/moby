//go:build !windows

package daemon

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/plugin"
	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd/shimopts"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/runtime-spec/specs-go/features"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultRuntimeName = "runc"
)

type shimConfig struct {
	Shim     string
	Opts     interface{}
	Features *features.Features

	// Check if the ShimConfig is valid given the current state of the system.
	PreflightCheck func() error
}

type runtimes struct {
	configured map[string]*shimConfig
}

func configureRuntimes(conf *config.Config) {
	if conf.DefaultRuntime == "" {
		conf.DefaultRuntime = config.StockRuntimeName
	}
	if conf.Runtimes == nil {
		conf.Runtimes = make(map[string]types.Runtime)
	}
	conf.Runtimes[config.LinuxV2RuntimeName] = types.Runtime{Path: defaultRuntimeName}
	conf.Runtimes[config.StockRuntimeName] = conf.Runtimes[config.LinuxV2RuntimeName]
}

func defaultV2ShimConfig(conf *config.Config, runtimePath string) *shimConfig {
	shim := &shimConfig{
		Shim: plugin.RuntimeRuncV2,
		Opts: &v2runcoptions.Options{
			BinaryName:    runtimePath,
			Root:          filepath.Join(conf.ExecRoot, "runtime-"+defaultRuntimeName),
			SystemdCgroup: UsingSystemd(conf),
			NoPivotRoot:   os.Getenv("DOCKER_RAMDISK") != "",
		},
	}

	var featuresStderr bytes.Buffer
	featuresCmd := exec.Command(runtimePath, "features")
	featuresCmd.Stderr = &featuresStderr
	if featuresB, err := featuresCmd.Output(); err != nil {
		logrus.WithError(err).Warnf("Failed to run %v: %q", featuresCmd.Args, featuresStderr.String())
	} else {
		var features features.Features
		if jsonErr := json.Unmarshal(featuresB, &features); jsonErr != nil {
			logrus.WithError(err).Warnf("Failed to unmarshal the output of %v as a JSON", featuresCmd.Args)
		} else {
			shim.Features = &features
		}
	}

	return shim
}

func runtimeScriptsDir(cfg *config.Config) string {
	return filepath.Join(cfg.Root, "runtimes")
}

// initRuntimesDir creates a fresh directory where we'll store the runtime
// scripts (i.e. in order to support runtimeArgs).
func initRuntimesDir(cfg *config.Config) error {
	runtimeDir := runtimeScriptsDir(cfg)
	if err := os.RemoveAll(runtimeDir); err != nil {
		return err
	}
	return system.MkdirAll(runtimeDir, 0700)
}

func setupRuntimes(cfg *config.Config) (runtimes, error) {
	newrt := runtimes{
		configured: make(map[string]*shimConfig),
	}

	dir := runtimeScriptsDir(cfg)
	for name, rt := range cfg.Runtimes {
		var c *shimConfig
		if rt.Path == "" && rt.Type == "" {
			return runtimes{}, errors.Errorf("runtime %s: either a runtimeType or a path must be configured", name)
		}
		if rt.Path != "" {
			if rt.Type != "" {
				return runtimes{}, errors.Errorf("runtime %s: cannot configure both path and runtimeType for the same runtime", name)
			}
			if len(rt.Options) > 0 {
				return runtimes{}, errors.Errorf("runtime %s: options cannot be used with a path runtime", name)
			}

			binaryName := rt.Path
			needsWrapper := len(rt.Args) > 0
			if needsWrapper {
				var err error
				binaryName, err = wrapRuntime(dir, name, rt.Path, rt.Args)
				if err != nil {
					return runtimes{}, err
				}
			}
			c = defaultV2ShimConfig(cfg, binaryName)
			if needsWrapper {
				path := rt.Path
				c.PreflightCheck = func() error {
					// Check that the runtime path actually exists so that we can return a well known error.
					_, err := exec.LookPath(path)
					return errors.Wrap(err, "error while looking up the specified runtime path")
				}
			}
		} else {
			if len(rt.Args) > 0 {
				return runtimes{}, errors.Errorf("runtime %s: args cannot be used with a runtimeType runtime", name)
			}
			// Unlike implicit runtimes, there is no restriction on configuring a shim by path.
			c = &shimConfig{Shim: rt.Type}
			if len(rt.Options) > 0 {
				// It has to be a pointer type or there'll be a panic in containerd/typeurl when we try to start the container.
				var err error
				c.Opts, err = shimopts.Generate(rt.Type, rt.Options)
				if err != nil {
					return runtimes{}, errors.Wrapf(err, "runtime %v", name)
				}
			}
		}
		newrt.configured[name] = c
	}

	return newrt, nil
}

// A non-standard Base32 encoding which lacks vowels to avoid accidentally
// spelling naughty words. Don't use this to encode any data which requires
// compatibility with anything outside of the currently-running process.
var base32Disemvoweled = base32.NewEncoding("0123456789BCDFGHJKLMNPQRSTVWXYZ-")

// wrapRuntime writes a shell script to dir which will execute binary with args
// concatenated to the script's argv. This is needed because the
// io.containerd.runc.v2 shim has no options for passing extra arguments to the
// runtime binary.
func wrapRuntime(dir, name, binary string, args []string) (string, error) {
	var wrapper bytes.Buffer
	sum := sha256.New()
	_, _ = fmt.Fprintf(io.MultiWriter(&wrapper, sum), "#!/bin/sh\n%s %s $@\n", binary, strings.Join(args, " "))
	// Generate a consistent name for the wrapper script derived from the
	// contents so that multiple wrapper scripts can coexist with the same
	// base name. The existing scripts might still be referenced by running
	// containers.
	suffix := base32Disemvoweled.EncodeToString(sum.Sum(nil))
	scriptPath := filepath.Join(dir, name+"."+suffix)
	if err := ioutils.AtomicWriteFile(scriptPath, wrapper.Bytes(), 0700); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func (r *runtimes) Get(name string) (string, interface{}, error) {
	rt := r.configured[name]
	if rt != nil {
		if rt.PreflightCheck != nil {
			if err := rt.PreflightCheck(); err != nil {
				return "", nil, err
			}
		}
		return rt.Shim, rt.Opts, nil
	}

	if !config.IsPermissibleC8dRuntimeName(name) {
		return "", nil, errdefs.InvalidParameter(errors.Errorf("unknown or invalid runtime name: %s", name))
	}
	return name, nil, nil
}

func (r *runtimes) Features(name string) *features.Features {
	rt := r.configured[name]
	if rt != nil {
		return rt.Features
	}
	return nil
}
