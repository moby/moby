/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// FromOCILinuxSeccomp converts an seccomp configuration from an OCI runtime spec.
func FromOCILinuxSeccomp(o *rspec.LinuxSeccomp) *LinuxSeccomp {
	var errno *OptionalUInt32
	if o.DefaultErrnoRet != nil {
		errno = &OptionalUInt32{Value: uint32(*o.DefaultErrnoRet)}
	}

	arches := make([]string, len(o.Architectures))
	for i, arch := range o.Architectures {
		arches[i] = string(arch)
	}

	flags := make([]string, len(o.Flags))
	for i, flag := range o.Flags {
		flags[i] = string(flag)
	}

	return &LinuxSeccomp{
		DefaultAction:    string(o.DefaultAction),
		DefaultErrno:     errno,
		Architectures:    arches,
		Flags:            flags,
		ListenerPath:     o.ListenerPath,
		ListenerMetadata: o.ListenerMetadata,
		Syscalls:         FromOCILinuxSyscalls(o.Syscalls),
	}
}

// FromOCILinuxSyscalls converts seccomp syscalls configuration from an OCI runtime spec.
func FromOCILinuxSyscalls(o []rspec.LinuxSyscall) []*LinuxSyscall {
	syscalls := make([]*LinuxSyscall, len(o))

	for i, syscall := range o {
		var errno *OptionalUInt32
		if syscall.ErrnoRet != nil {
			errno = &OptionalUInt32{Value: uint32(*syscall.ErrnoRet)}
		}

		syscalls[i] = &LinuxSyscall{
			Names:    syscall.Names,
			Action:   string(syscall.Action),
			ErrnoRet: errno,
			Args:     FromOCILinuxSeccompArgs(syscall.Args),
		}
	}

	return syscalls
}

// FromOCILinuxSeccompArgs converts seccomp syscall args from an OCI runtime spec.
func FromOCILinuxSeccompArgs(o []rspec.LinuxSeccompArg) []*LinuxSeccompArg {
	args := make([]*LinuxSeccompArg, len(o))

	for i, arg := range o {
		args[i] = &LinuxSeccompArg{
			Index:    uint32(arg.Index),
			Value:    arg.Value,
			ValueTwo: arg.ValueTwo,
			Op:       string(arg.Op),
		}
	}

	return args
}

// ToOCILinuxSyscalls converts seccomp syscalls configuration to an OCI runtime spec.
func ToOCILinuxSyscalls(o []*LinuxSyscall) []rspec.LinuxSyscall {
	syscalls := make([]rspec.LinuxSyscall, len(o))

	for i, syscall := range o {
		var errnoRet *uint

		if syscall.ErrnoRet != nil {
			*errnoRet = uint(syscall.ErrnoRet.Value)
		}

		syscalls[i] = rspec.LinuxSyscall{
			Names:    syscall.Names,
			Action:   rspec.LinuxSeccompAction(syscall.Action),
			ErrnoRet: errnoRet,
			Args:     ToOCILinuxSeccompArgs(syscall.Args),
		}
	}

	return syscalls
}

// ToOCILinuxSeccompArgs converts seccomp syscall args to an OCI runtime spec.
func ToOCILinuxSeccompArgs(o []*LinuxSeccompArg) []rspec.LinuxSeccompArg {
	args := make([]rspec.LinuxSeccompArg, len(o))

	for i, arg := range o {
		args[i] = rspec.LinuxSeccompArg{
			Index:    uint(arg.Index),
			Value:    arg.Value,
			ValueTwo: arg.ValueTwo,
			Op:       rspec.LinuxSeccompOperator(arg.Op),
		}
	}

	return args
}
