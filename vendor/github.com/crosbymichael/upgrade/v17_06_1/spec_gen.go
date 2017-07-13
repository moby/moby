// DO NOT EDIT
// This file has been auto-generated with go generate.

package v17_06_1

import specs "github.com/opencontainers/runtime-spec/specs-go" // a45ba0989fc26c695fe166a49c45bb8b7618ab36 https://github.com/docker/runtime-spec

type Spec struct {
	Version  string         `json:"ociVersion"`
	Platform specs.Platform `json:"platform"`
	Process  struct {
		Terminal        bool                `json:"terminal,omitempty"`
		ConsoleSize     specs.Box           `json:"consoleSize,omitempty"`
		User            specs.User          `json:"user"`
		Args            []string            `json:"args"`
		Env             []string            `json:"env,omitempty"`
		Cwd             string              `json:"cwd"`
		Capabilities    linuxCapabilities   `json:"capabilities,omitempty" platform:"linux"`
		Rlimits         []specs.LinuxRlimit `json:"rlimits,omitempty" platform:"linux"`
		NoNewPrivileges bool                `json:"noNewPrivileges,omitempty" platform:"linux"`
		ApparmorProfile string              `json:"apparmorProfile,omitempty" platform:"linux"`
		SelinuxLabel    string              `json:"selinuxLabel,omitempty" platform:"linux"`
	} `json:"process"`
	Root        specs.Root        `json:"root"`
	Hostname    string            `json:"hostname,omitempty"`
	Mounts      []specs.Mount     `json:"mounts,omitempty"`
	Hooks       *specs.Hooks      `json:"hooks,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Linux       *struct {
		UIDMappings []specs.LinuxIDMapping `json:"uidMappings,omitempty"`
		GIDMappings []specs.LinuxIDMapping `json:"gidMappings,omitempty"`
		Sysctl      map[string]string      `json:"sysctl,omitempty"`
		Resources   *struct {
			Devices          []specs.LinuxDeviceCgroup `json:"devices,omitempty"`
			DisableOOMKiller *bool                     `json:"disableOOMKiller,omitempty"`
			OOMScoreAdj      *int                      `json:"oomScoreAdj,omitempty"`
			Memory           *struct {
				Limit       *int64           `json:"limit,omitempty"`
				Reservation *int64           `json:"reservation,omitempty"`
				Swap        *int64           `json:"swap,omitempty"`
				Kernel      *int64           `json:"kernel,omitempty"`
				KernelTCP   *int64           `json:"kernelTCP,omitempty"`
				Swappiness  memorySwappiness `json:"swappiness,omitempty"`
			} `json:"memory,omitempty"`
			CPU            *specs.LinuxCPU            `json:"cpu,omitempty"`
			Pids           *specs.LinuxPids           `json:"pids,omitempty"`
			BlockIO        *specs.LinuxBlockIO        `json:"blockIO,omitempty"`
			HugepageLimits []specs.LinuxHugepageLimit `json:"hugepageLimits,omitempty"`
			Network        *specs.LinuxNetwork        `json:"network,omitempty"`
		} `json:"resources,omitempty"`
		CgroupsPath string                 `json:"cgroupsPath,omitempty"`
		Namespaces  []specs.LinuxNamespace `json:"namespaces,omitempty"`
		Devices     []specs.LinuxDevice    `json:"devices,omitempty"`
		Seccomp     *struct {
			DefaultAction specs.LinuxSeccompAction `json:"defaultAction"`
			Architectures []specs.Arch             `json:"architectures,omitempty"`
			Syscalls      linuxSyscalls            `json:"syscalls"`
		} `json:"seccomp,omitempty"`
		RootfsPropagation string   `json:"rootfsPropagation,omitempty"`
		MaskedPaths       []string `json:"maskedPaths,omitempty"`
		ReadonlyPaths     []string `json:"readonlyPaths,omitempty"`
		MountLabel        string   `json:"mountLabel,omitempty"`
	} `json:"linux,omitempty" platform:"linux"`
	Solaris *specs.Solaris `json:"solaris,omitempty" platform:"solaris"`
	Windows *specs.Windows `json:"windows,omitempty" platform:"windows"`
}
