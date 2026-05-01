package seccomp // import "github.com/docker/docker/profiles/seccomp"

import (
	"github.com/moby/profiles/seccomp"
)

// Seccomp represents the config for a seccomp profile for syscall restriction.
// It is used to marshal/unmarshal the JSON profiles as accepted by docker, and
// extends the runtime-spec's specs.LinuxSeccomp, overriding some fields to
// provide the ability to define conditional rules based on the host's kernel
// version, architecture, and the container's capabilities.
//
//go:fix inline
type Seccomp = seccomp.Seccomp

// Architecture is used to represent a specific architecture
// and its sub-architectures
//
//go:fix inline
type Architecture = seccomp.Architecture

// Filter is used to conditionally apply Seccomp rules
//
//go:fix inline
type Filter = seccomp.Filter

// Syscall is used to match a group of syscalls in Seccomp. It extends the
// runtime-spec Syscall type, adding a "Name" field for backward compatibility
// with older JSON representations, additional "Comment" metadata, and conditional
// rules ("Includes", "Excludes") used to generate a runtime-spec Seccomp profile
// based on the container (capabilities) and host's (arch, kernel) configuration.
//
//go:fix inline
type Syscall = seccomp.Syscall

// KernelVersion holds information about the kernel.
//
//go:fix inline
type KernelVersion = seccomp.KernelVersion
