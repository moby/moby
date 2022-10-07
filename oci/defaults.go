package oci // import "github.com/docker/docker/oci"

import (
	"runtime"

	"github.com/docker/docker/oci/caps"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func iPtr(i int64) *int64 { return &i }

const defaultUnixPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// DefaultPathEnv is unix style list of directories to search for
// executables. Each directory is separated from the next by a colon
// ':' character .
// For Windows containers, an empty string is returned as the default
// path will be set by the container, and Docker has no context of what the
// default path should be.
//
// TODO(thaJeztah) align Windows default with BuildKit; see https://github.com/moby/buildkit/pull/1747
// TODO(thaJeztah) use defaults from containerd (but align it with BuildKit; see https://github.com/moby/buildkit/pull/1747)
func DefaultPathEnv(os string) string {
	if os == "windows" {
		return ""
	}
	return defaultUnixPathEnv
}

// DefaultSpec returns the default spec used by docker for the current Platform
func DefaultSpec() specs.Spec {
	if runtime.GOOS == "windows" {
		return DefaultWindowsSpec()
	}
	return DefaultLinuxSpec()
}

// DefaultWindowsSpec create a default spec for running Windows containers
func DefaultWindowsSpec() specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Windows: &specs.Windows{},
		Process: &specs.Process{},
		Root:    &specs.Root{},
	}
}

// DefaultLinuxSpec create a default spec for running Linux containers
func DefaultLinuxSpec() specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Capabilities: &specs.LinuxCapabilities{
				Bounding:  caps.DefaultCapabilities(),
				Permitted: caps.DefaultCapabilities(),
				Effective: caps.DefaultCapabilities(),
			},
		},
		Root: &specs.Root{},
		Mounts: []specs.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
			{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"ro", "nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777"},
			},
		},
		Linux: &specs.Linux{
			MaskedPaths: []string{
				"/proc/asound",
				"/proc/acpi",
				"/proc/kcore",
				"/proc/keys",
				"/proc/latency_stats",
				"/proc/timer_list",
				"/proc/timer_stats",
				"/proc/sched_debug",
				"/proc/scsi",
				"/sys/firmware",
			},
			ReadonlyPaths: []string{
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
			Namespaces: []specs.LinuxNamespace{
				{Type: "mount"},
				{Type: "network"},
				{Type: "uts"},
				{Type: "pid"},
				{Type: "ipc"},
			},
			// Devices implicitly contains the following devices:
			// null, zero, full, random, urandom, tty, console, and ptmx.
			// ptmx is a bind mount or symlink of the container's ptmx.
			// See also: https://github.com/opencontainers/runtime-spec/blob/master/config-linux.md#default-devices
			Devices: []specs.LinuxDevice{},
			Resources: &specs.LinuxResources{
				Devices: []specs.LinuxDeviceCgroup{
					{
						Allow:  false,
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(1),
						Minor:  iPtr(5),
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(1),
						Minor:  iPtr(3),
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(1),
						Minor:  iPtr(9),
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(1),
						Minor:  iPtr(8),
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(5),
						Minor:  iPtr(0),
						Access: "rwm",
					},
					{
						Allow:  true,
						Type:   "c",
						Major:  iPtr(5),
						Minor:  iPtr(1),
						Access: "rwm",
					},
					{
						Allow:  false,
						Type:   "c",
						Major:  iPtr(10),
						Minor:  iPtr(229),
						Access: "rwm",
					},
				},
			},
		},
	}
}
