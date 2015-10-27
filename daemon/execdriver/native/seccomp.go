// +build linux

package native

import (
	"encoding/json"
	"fmt"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"io/ioutil"
	"os"
)

// Seccomp represents syscall restrictions
type Seccomp struct {
	DefaultAction Action     `json:"defaultAction"`
	Architectures []Arch     `json:"architectures"`
	Syscalls      []*Syscall `json:"syscalls"`
}

// Arch permitted to be used for system calls
// By default only the native architecture of the kernel is permitted
type Arch string

// Action taken upon Seccomp rule match
type Action string

// Operator used to match syscall arguments in Seccomp
type Operator string

// Arg used for matching specific syscall arguments in Seccomp
type Arg struct {
	Index    uint     `json:"index"`
	Value    uint64   `json:"value"`
	ValueTwo uint64   `json:"valueTwo"`
	Op       Operator `json:"op"`
}

// Syscall is used to match a syscall in Seccomp
type Syscall struct {
	Name   string `json:"name"`
	Action Action `json:"action"`
	Args   []*Arg `json:"args"`
}

func temlateSeccomp(path string) error {
	config := Seccomp{
		DefaultAction: "SCMP_ACT_ALLOW",
		Syscalls: []*Syscall{
			{
				Name:   "unshare",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "setns",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "mount",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "umount2",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "create_module",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "delete_module",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "chmod",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "chown",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "link",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "linkat",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "unlink",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "unlinkat",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "chroot",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "sethostname",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "setdomainname",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "kexec_load",
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "unlink",
				Action: "SCMP_ACT_ERRNO",
			},
		},
	}

	data, err := json.MarshalIndent(&config, "", "\t")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path, data, 0666); err != nil {
		return err
	}
	return nil
}

func loadSeccomp(path string) (*configs.Seccomp, error) {
	if _, err := os.Stat(path); err != nil {
		if err := temlateSeccomp(path); err != nil {
			return nil, fmt.Errorf("genSeccomp at %s failed", path)
		}
	}
	sf, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open seccomp config file at %s failed", path)
	}
	defer sf.Close()

	config := Seccomp{}
	if err = json.NewDecoder(sf).Decode(&config); err != nil {
		return nil, fmt.Errorf("seccomp decode at %s failed", path)
	}

	// No default action specified, no syscalls listed, assume seccomp disabled
	if config.DefaultAction == "" && len(config.Syscalls) == 0 {
		return nil, nil
	}

	newConfig := new(configs.Seccomp)
	newConfig.Syscalls = []*configs.Syscall{}

	// Convert default action from string representation
	newDefaultAction, err := seccomp.ConvertStringToAction(string(config.DefaultAction))
	if err != nil {
		return nil, err
	}
	newConfig.DefaultAction = newDefaultAction

	// Loop through all syscall blocks and convert them to libcontainer format
	for _, call := range config.Syscalls {
		newAction, err := seccomp.ConvertStringToAction(string(call.Action))
		if err != nil {
			return nil, err
		}

		newCall := configs.Syscall{
			Name:   call.Name,
			Action: newAction,
			Args:   []*configs.Arg{},
		}

		// Loop through all the arguments of the syscall and convert them
		for _, arg := range call.Args {
			newOp, err := seccomp.ConvertStringToOperator(string(arg.Op))
			if err != nil {
				return nil, err
			}

			newArg := configs.Arg{
				Index:    arg.Index,
				Value:    arg.Value,
				ValueTwo: arg.ValueTwo,
				Op:       newOp,
			}

			newCall.Args = append(newCall.Args, &newArg)
		}

		newConfig.Syscalls = append(newConfig.Syscalls, &newCall)
	}

	return newConfig, nil
}
