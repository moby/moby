package seccomp // import "github.com/docker/docker/profiles/seccomp"

import "github.com/opencontainers/runtime-spec/specs-go"

// Seccomp represents the config for a seccomp profile for syscall restriction.
type Seccomp struct {
	DefaultAction specs.LinuxSeccompAction `json:"defaultAction"`
	// Architectures is kept to maintain backward compatibility with the old
	// seccomp profile.
	Architectures []specs.Arch   `json:"architectures,omitempty"`
	ArchMap       []Architecture `json:"archMap,omitempty"`
	Syscalls      []*Syscall     `json:"syscalls"`
}

// Architecture is used to represent a specific architecture
// and its sub-architectures
type Architecture struct {
	Arch      specs.Arch   `json:"architecture"`
	SubArches []specs.Arch `json:"subArchitectures"`
}

// Filter is used to conditionally apply Seccomp rules
type Filter struct {
	Caps      []string `json:"caps,omitempty"`
	Arches    []string `json:"arches,omitempty"`
	MinKernel string   `json:"minKernel,omitempty"`
}

// Syscall is used to match a group of syscalls in Seccomp
type Syscall struct {
	Name     string                   `json:"name,omitempty"`
	Names    []string                 `json:"names,omitempty"`
	Action   specs.LinuxSeccompAction `json:"action"`
	Args     []*specs.LinuxSeccompArg `json:"args"`
	Comment  string                   `json:"comment"`
	Includes Filter                   `json:"includes"`
	Excludes Filter                   `json:"excludes"`
}
