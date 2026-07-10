// Package features provides the Features struct.
package features

// Features represents the supported features of the runtime.
type Features struct {
	// OCIVersionMin is the minimum OCI Runtime Spec version recognized by the runtime, e.g., "1.0.0".
	OCIVersionMin string `json:"ociVersionMin,omitempty"`

	// OCIVersionMax is the maximum OCI Runtime Spec version recognized by the runtime, e.g., "1.0.2-dev".
	OCIVersionMax string `json:"ociVersionMax,omitempty"`

	// Hooks is the list of the recognized hook names, e.g., "createRuntime".
	// Nil value means "unknown", not "no support for any hook".
	Hooks []string `json:"hooks,omitempty"`

	// MountOptions is the list of the recognized mount options, e.g., "ro".
	// Nil value means "unknown", not "no support for any mount option".
	// This list does not contain filesystem-specific options passed to mount(2) syscall as (const void *).
	MountOptions []string `json:"mountOptions,omitempty"`

	// Linux is specific to Linux.
	Linux *Linux `json:"linux,omitempty"`

	// Annotations contains implementation-specific annotation strings,
	// such as the implementation version, and third-party extensions.
	Annotations map[string]string `json:"annotations,omitempty"`

	// PotentiallyUnsafeConfigAnnotations the list of the potential unsafe annotations
	// that may appear in `config.json`.
	//
	// A value that ends with "." is interpreted as a prefix of annotations.
	PotentiallyUnsafeConfigAnnotations []string `json:"potentiallyUnsafeConfigAnnotations,omitempty"`
}

// Linux is specific to Linux.
type Linux struct {
	// Namespaces is the list of the recognized namespaces, e.g., "mount".
	// Nil value means "unknown", not "no support for any namespace".
	Namespaces []string `json:"namespaces,omitempty"`

	// Capabilities is the list of the recognized capabilities , e.g., "CAP_SYS_ADMIN".
	// Nil value means "unknown", not "no support for any capability".
	Capabilities []string `json:"capabilities,omitempty"`

	Cgroup          *Cgroup          `json:"cgroup,omitempty"`
	Seccomp         *Seccomp         `json:"seccomp,omitempty"`
	Apparmor        *Apparmor        `json:"apparmor,omitempty"`
	Selinux         *Selinux         `json:"selinux,omitempty"`
	IntelRdt        *IntelRdt        `json:"intelRdt,omitempty"`
	MountExtensions *MountExtensions `json:"mountExtensions,omitempty"`
}

// Cgroup represents the "cgroup" field.
type Cgroup struct {
	// V1 represents whether Cgroup v1 support is compiled in.
	// Unrelated to whether the host uses cgroup v1 or not.
	// Nil value means "unknown", not "false".
	V1 *bool `json:"v1,omitempty"`

	// V2 represents whether Cgroup v2 support is compiled in.
	// Unrelated to whether the host uses cgroup v2 or not.
	// Nil value means "unknown", not "false".
	V2 *bool `json:"v2,omitempty"`

	// Systemd represents whether systemd-cgroup support is compiled in.
	// Unrelated to whether the host uses systemd or not.
	// Nil value means "unknown", not "false".
	Systemd *bool `json:"systemd,omitempty"`

	// SystemdUser represents whether user-scoped systemd-cgroup support is compiled in.
	// Unrelated to whether the host uses systemd or not.
	// Nil value means "unknown", not "false".
	SystemdUser *bool `json:"systemdUser,omitempty"`

	// Rdma represents whether RDMA cgroup support is compiled in.
	// Unrelated to whether the host supports RDMA or not.
	// Nil value means "unknown", not "false".
	Rdma *bool `json:"rdma,omitempty"`
}

// Seccomp represents the "seccomp" field.
type Seccomp struct {
	// Enabled is true if seccomp support is compiled in.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`

	// Actions is the list of the recognized actions, e.g., "SCMP_ACT_NOTIFY".
	// Nil value means "unknown", not "no support for any action".
	Actions []string `json:"actions,omitempty"`

	// Operators is the list of the recognized operators, e.g., "SCMP_CMP_NE".
	// Nil value means "unknown", not "no support for any operator".
	Operators []string `json:"operators,omitempty"`

	// Archs is the list of the recognized archs, e.g., "SCMP_ARCH_X86_64".
	// Nil value means "unknown", not "no support for any arch".
	Archs []string `json:"archs,omitempty"`

	// KnownFlags is the list of the recognized filter flags, e.g., "SECCOMP_FILTER_FLAG_LOG".
	// Nil value means "unknown", not "no flags are recognized".
	KnownFlags []string `json:"knownFlags,omitempty"`

	// SupportedFlags is the list of the supported filter flags, e.g., "SECCOMP_FILTER_FLAG_LOG".
	// This list may be a subset of KnownFlags due to some flags
	// not supported by the current kernel and/or libseccomp.
	// Nil value means "unknown", not "no flags are supported".
	SupportedFlags []string `json:"supportedFlags,omitempty"`
}

// Apparmor represents the "apparmor" field.
type Apparmor struct {
	// Enabled is true if AppArmor support is compiled in.
	// Unrelated to whether the host supports AppArmor or not.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`
}

// Selinux represents the "selinux" field.
type Selinux struct {
	// Enabled is true if SELinux support is compiled in.
	// Unrelated to whether the host supports SELinux or not.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`
}

// IntelRdt represents the "intelRdt" field.
type IntelRdt struct {
	// Enabled is true if Intel RDT support is compiled in.
	// Unrelated to whether the host supports Intel RDT or not.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`
}

// MountExtensions represents the "mountExtensions" field.
type MountExtensions struct {
	// IDMap represents the status of idmap mounts support.
	IDMap *IDMap `json:"idmap,omitempty"`
}

type IDMap struct {
	// Enabled represents whether idmap mounts supports is compiled in.
	// Unrelated to whether the host supports it or not.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`
}
