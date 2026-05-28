// Package modprobe attempts to load kernel modules. It may have more success
// than simply running "modprobe", particularly for docker-in-docker.
package modprobe

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/containerd/log"
	"golang.org/x/sys/unix"
)

// LoadModules attempts to load kernel modules, if necessary.
//
// isLoaded must be a function that checks whether the modules are loaded. It may
// be called multiple times. isLoaded must return an error to indicate that the
// modules still need to be loaded, otherwise nil.
//
// For each method of loading modules, LoadModules will attempt the load for each
// of modNames, then it will call isLoaded to check the result - moving on to try
// the next method if needed, and there is one.
//
// The returned error is the result of the final call to isLoaded.
func LoadModules(ctx context.Context, isLoaded func() error, modNames ...string) error {
	if isLoaded() == nil {
		log.G(ctx).WithFields(log.Fields{
			"modules": modNames,
		}).Debug("Modules already loaded")
		return nil
	}

	if err := tryLoad(ctx, isLoaded, modNames, ioctlLoader{}); err != nil {
		return tryLoad(ctx, isLoaded, modNames, modprobeLoader{})
	}
	return nil
}

type loader interface {
	name() string
	load(modName string) error
}

func tryLoad(ctx context.Context, isLoaded func() error, modNames []string, loader loader) error {
	var loadErrs []error
	for _, modName := range modNames {
		if err := loader.load(modName); err != nil {
			loadErrs = append(loadErrs, err)
		}
	}

	if checkResult := isLoaded(); checkResult != nil {
		log.G(ctx).WithFields(log.Fields{
			"loader":      loader.name(),
			"modules":     modNames,
			"loadErrors":  errors.Join(loadErrs...),
			"checkResult": checkResult,
		}).Debug("Modules not loaded")
		return checkResult
	}

	log.G(ctx).WithFields(log.Fields{
		"loader":     loader.name(),
		"modules":    modNames,
		"loadErrors": errors.Join(loadErrs...),
	}).Debug("Modules loaded")
	return nil
}

// ioctlLoader attempts to load the module using an ioctl() to get the interface index
// of a module - it won't have one, but the kernel may load the module. This tends to
// work in docker-in-docker, where the inner-docker may not have "modprobe" or access
// to modules in the host's filesystem.
type ioctlLoader struct{}

func (il ioctlLoader) name() string { return "ioctl" }

func (il ioctlLoader) load(modName string) error {
	sd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("creating socket for ioctl load of %s: %w", modName, err)
	}
	defer unix.Close(sd)

	// This tends to work, if running with CAP_SYS_MODULE, because...
	//   https://github.com/torvalds/linux/blob/6f7da290413ba713f0cdd9ff1a2a9bb129ef4f6c/net/core/dev_ioctl.c#L457
	//   https://github.com/torvalds/linux/blob/6f7da290413ba713f0cdd9ff1a2a9bb129ef4f6c/net/core/dev_ioctl.c#L371-L372
	ifreq, err := unix.NewIfreq(modName)
	if err != nil {
		return fmt.Errorf("creating ifreq for %s: %w", modName, err)
	}
	// An error is returned even if the module load is successful. So, ignore it.
	_ = unix.IoctlIfreq(sd, unix.SIOCGIFINDEX, ifreq)
	return nil
}

// modprobeLoader attempts to load a kernel module using modprobe.
type modprobeLoader struct{}

func (ml modprobeLoader) name() string { return "modprobe" }

func (ml modprobeLoader) load(modName string) error {
	out, err := exec.Command("modprobe", "-va", modName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("modprobe %s failed with message: %q, error: %w", modName, strings.TrimSpace(string(out)), err)
	}
	return nil
}
