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


	if len(config.Architectures) != 0 && len(config.ArchMap) != 0 {
		return nil, errors.New("'architectures' and 'archMap' were specified in the seccomp profile, use either 'architectures' or 'archMap'")
	}

	// if config.Architectures == 0 then libseccomp will figure out the architecture to use
	if len(config.Architectures) != 0 {
		for _, a := range config.Architectures {
			newConfig.Architectures = append(newConfig.Architectures, specs.Arch(a))
		}
	}

	if len(config.ArchMap) != 0 {
		for _, a := range config.ArchMap {
			if a.Arch == seccompArch {
				newConfig.Architectures = append(newConfig.Architectures, specs.Arch(a.Arch))
				for _, sa := range a.SubArches {
					newConfig.Architectures = append(newConfig.Architectures, specs.Arch(sa))
				}
				break
			}
		}
	}

	newConfig.DefaultAction = specs.LinuxSeccompAction(config.DefaultAction)

Loop:
	// Loop through all syscall blocks and convert them to libcontainer format after filtering them
	for _, call := range config.Syscalls {
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

		if call.Name != "" && len(call.Names) != 0 {
			return nil, errors.New("'name' and 'names' were specified in the seccomp profile, use either 'name' or 'names'")
		}

		if call.Name != "" {
			newConfig.Syscalls = append(newConfig.Syscalls, createSpecsSyscall(call.Name, call.Action, call.Args))
		}

		for _, n := range call.Names {
			newConfig.Syscalls = append(newConfig.Syscalls, createSpecsSyscall(n, call.Action, call.Args))
		}
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
