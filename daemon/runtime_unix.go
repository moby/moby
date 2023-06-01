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
	"github.com/containerd/containerd/runtime/v2/shim"
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

	// The runtime used to specify the containerd v2 runc shim
	linuxV2RuntimeName = "io.containerd.runc.v2"
)

type shimConfig struct {
	Shim     string
	Opts     interface{}
	Features *features.Features

	// Check if the ShimConfig is valid given the current state of the system.
	PreflightCheck func() error
}

type runtimes struct {
	Default    string
	configured map[string]*shimConfig
}

func stockRuntimes() map[string]string {
	return map[string]string{
		linuxV2RuntimeName:      defaultRuntimeName,
		config.StockRuntimeName: defaultRuntimeName,
	}
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
	if _, ok := cfg.Runtimes[config.StockRuntimeName]; ok {
		return runtimes{}, errors.Errorf("runtime name '%s' is reserved", config.StockRuntimeName)
	}

	newrt := runtimes{
		Default:    cfg.DefaultRuntime,
		configured: make(map[string]*shimConfig),
	}
	for name, path := range stockRuntimes() {
		newrt.configured[name] = defaultV2ShimConfig(cfg, path)
	}

	if newrt.Default != "" {
		_, isStock := newrt.configured[newrt.Default]
		_, isConfigured := cfg.Runtimes[newrt.Default]
		if !isStock && !isConfigured && !isPermissibleC8dRuntimeName(newrt.Default) {
			return runtimes{}, errors.Errorf("specified default runtime '%s' does not exist", newrt.Default)
		}
	} else {
		newrt.Default = config.StockRuntimeName
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

// Get returns the containerd runtime and options for name, suitable to pass
// into containerd.WithRuntime(). The runtime and options for the default
// runtime are returned when name is the empty string.
func (r *runtimes) Get(name string) (string, interface{}, error) {
	if name == "" {
		name = r.Default
	}

	rt := r.configured[name]
	if rt != nil {
		if rt.PreflightCheck != nil {
			if err := rt.PreflightCheck(); err != nil {
				return "", nil, err
			}
		}
		return rt.Shim, rt.Opts, nil
	}

	if !isPermissibleC8dRuntimeName(name) {
		return "", nil, errdefs.InvalidParameter(errors.Errorf("unknown or invalid runtime name: %s", name))
	}
	return name, nil, nil
}

func (r *runtimes) Features(name string) *features.Features {
	if name == "" {
		name = r.Default
	}

	rt := r.configured[name]
	if rt != nil {
		return rt.Features
	}
	return nil
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
