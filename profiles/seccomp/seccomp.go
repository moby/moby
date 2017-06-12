// +build linux

package seccomp

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/opencontainers/runtime-spec/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
)

//go:generate go run -tags 'seccomp' generate.go

// GetDefaultProfile returns the default seccomp profile.
func GetDefaultProfile(rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return setupSeccomp(DefaultProfile(), rs)
}

// LoadProfile takes a json string and decodes the seccomp profile.
func LoadProfile(body string, rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	var config types.Seccomp
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		return nil, fmt.Errorf("Decoding seccomp profile failed: %v", err)
	}
	return setupSeccomp(&config, rs)
}

var nativeToSeccomp = map[string]types.Arch{
	"amd64":       types.ArchX86_64,
	"arm64":       types.ArchAARCH64,
	"mips64":      types.ArchMIPS64,
	"mips64n32":   types.ArchMIPS64N32,
	"mipsel64":    types.ArchMIPSEL64,
	"mipsel64n32": types.ArchMIPSEL64N32,
	"s390x":       types.ArchS390X,
}

// Returns the architecture C libseccomp was compiled for
func getSeccompArch() (types.Arch, error) {
	if n, err := libseccomp.GetNativeArch(); err != nil {
		return "", fmt.Errorf("libseccomp internal error: %v", err)
	} else if seccompArch, ok := nativeToSeccomp[n.String()]; !ok {
		return "", fmt.Errorf("unknown architecture %q returned by libseccomp", n)
	} else {
		return seccompArch, nil
	}
}

// Converts { "amd64", "s390", "foobar" } into
// { "SCMP_ARCH_X86_64", "SCMP_ARCH_S390", "foobar" } etc.
// When no conversion was performed, the input slice itself is returned
func supportLegacyArchID(arr []string) []string {
	ret := &arr
	var allocated []string
	for i, inp := range arr {
		if a, ok := nativeToSeccomp[inp]; ok {
			if ret == &arr {
				allocated = make([]string, i, len(arr))
				copy(allocated, arr)
				ret = &allocated
			}
			sa := string(a)
			logrus.Warnf("seccomp: legacy arch ID %q should be replaced with %q",
				inp, sa)
			*ret = append(*ret, sa)
		} else if ret != &arr {
			*ret = append(*ret, inp)
		}
	}
	return *ret
}

func setupSeccomp(config *types.Seccomp, rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	if config == nil {
		return nil, nil
	}

	// No default action specified, no syscalls listed, assume seccomp disabled
	if config.DefaultAction == "" && len(config.Syscalls) == 0 {
		return nil, nil
	}

	newConfig := &specs.LinuxSeccomp{}

	seccompArch, err := getSeccompArch()
	if err != nil {
		return nil, err
	}

	registerArch := func(a types.Arch) {
		newConfig.Architectures = append(newConfig.Architectures, specs.Arch(a))
	}

	if len(config.Architectures) != 0 {
		if len(config.ArchMap) != 0 {
			return nil, errors.New("either one (not both) of 'architectures' or 'archMap' must be specified in the seccomp profile")
		}

		for _, a := range config.Architectures {
			registerArch(a)
		}
	} else {
		// libseccomp will figure out the architecture to use
		for _, a := range config.ArchMap {
			if a.Arch == seccompArch {
				registerArch(a.Arch)
				for _, sa := range a.SubArches {
					registerArch(sa)
				}
				break
			}
		}
	}

	newConfig.DefaultAction = specs.LinuxSeccompAction(config.DefaultAction)

Loop:
	// Loop through all syscall blocks and convert them to libcontainer format after filtering them
	for _, call := range config.Syscalls {
		registerSyscall := func(name string) {
			newConfig.Syscalls = append(newConfig.Syscalls, createSpecsSyscall(name, call.Action, call.Args))
		}

		if len(call.Excludes.Arches) > 0 {
			if stringutils.InSlice(supportLegacyArchID(call.Excludes.Arches), string(seccompArch)) {
				continue Loop
			}
		}
		if len(call.Excludes.Caps) > 0 {
			for _, c := range call.Excludes.Caps {
				if stringutils.InSlice(rs.Process.Capabilities.Effective, c) {
					continue Loop
				}
			}
		}
		if len(call.Includes.Arches) > 0 {
			if !stringutils.InSlice(supportLegacyArchID(call.Includes.Arches), string(seccompArch)) {
				continue Loop
			}
		}
		if len(call.Includes.Caps) > 0 {
			for _, c := range call.Includes.Caps {
				if !stringutils.InSlice(rs.Process.Capabilities.Effective, c) {
					continue Loop
				}
			}
		}

		if call.Name != "" {
			if len(call.Names) == 0 {
				registerSyscall(call.Name)
				continue Loop
			}
		} else {
			if len(call.Names) != 0 {
				for _, n := range call.Names {
					registerSyscall(n)
				}
				continue Loop
			}
		}

		return nil, fmt.Errorf("either one (not both) of 'name' or 'names' must be specified in the seccomp profile: %v", call)
	}

	return newConfig, nil
}

func createSpecsSyscall(name string, action types.Action, args []*types.Arg) specs.LinuxSyscall {
	newCall := specs.LinuxSyscall{
		Names:  []string{name},
		Action: specs.LinuxSeccompAction(action),
	}

	// Loop through all the arguments of the syscall and convert them
	for _, arg := range args {
		newArg := specs.LinuxSeccompArg{
			Index:    arg.Index,
			Value:    arg.Value,
			ValueTwo: arg.ValueTwo,
			Op:       specs.LinuxSeccompOperator(arg.Op),
		}

		newCall.Args = append(newCall.Args, newArg)
	}
	return newCall
}
