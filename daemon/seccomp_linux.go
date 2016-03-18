// +build linux,seccomp

package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/engine-api/types"
	"github.com/opencontainers/specs/specs-go"
)

func setSeccomp(daemon *Daemon, rs *specs.Spec, c *container.Container) error {
	var seccomp *specs.Seccomp
	var err error

	if c.HostConfig.Privileged {
		return nil
	}

	if !daemon.seccompEnabled {
		if c.SeccompProfile != "" && c.SeccompProfile != "unconfined" {
			return fmt.Errorf("Seccomp is not enabled in your kernel, cannot run a custom seccomp profile.")
		}
		logrus.Warn("Seccomp is not enabled in your kernel, running container without default profile.")
		c.SeccompProfile = "unconfined"
	}
	if c.SeccompProfile == "unconfined" {
		return nil
	}
	if c.SeccompProfile != "" {
		seccomp, err = loadSeccompProfile(c.SeccompProfile)
		if err != nil {
			return err
		}
	} else {
		seccomp = &defaultSeccompProfile
	}

	rs.Linux.Seccomp = seccomp
	return nil
}

func loadSeccompProfile(body string) (*specs.Seccomp, error) {
	var config types.Seccomp
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		return nil, fmt.Errorf("Decoding seccomp profile failed: %v", err)
	}

	return setupSeccomp(&config)
}

func setupSeccomp(config *types.Seccomp) (newConfig *specs.Seccomp, err error) {
	if config == nil {
		return nil, nil
	}

	// No default action specified, no syscalls listed, assume seccomp disabled
	if config.DefaultAction == "" && len(config.Syscalls) == 0 {
		return nil, nil
	}

	newConfig = &specs.Seccomp{}

	// if config.Architectures == 0 then libseccomp will figure out the architecture to use
	if len(config.Architectures) > 0 {
		// newConfig.Architectures = []string{}
		for _, arch := range config.Architectures {
			newConfig.Architectures = append(newConfig.Architectures, specs.Arch(arch))
		}
	}

	newConfig.DefaultAction = specs.Action(config.DefaultAction)

	// Loop through all syscall blocks and convert them to libcontainer format
	for _, call := range config.Syscalls {
		newCall := specs.Syscall{
			Name:   call.Name,
			Action: specs.Action(call.Action),
		}

		// Loop through all the arguments of the syscall and convert them
		for _, arg := range call.Args {
			newArg := specs.Arg{
				Index:    arg.Index,
				Value:    arg.Value,
				ValueTwo: arg.ValueTwo,
				Op:       specs.Operator(arg.Op),
			}

			newCall.Args = append(newCall.Args, newArg)
		}

		newConfig.Syscalls = append(newConfig.Syscalls, newCall)
	}

	return newConfig, nil
}
