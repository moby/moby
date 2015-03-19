package template

import (
	"syscall"

	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/configs"
)

const defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

// New returns the docker default configuration for libcontainer
func New() *configs.Config {
	container := &configs.Config{
		Capabilities: []string{
			"CHOWN",
			"DAC_OVERRIDE",
			"FSETID",
			"FOWNER",
			"MKNOD",
			"NET_RAW",
			"SETGID",
			"SETUID",
			"SETFCAP",
			"SETPCAP",
			"NET_BIND_SERVICE",
			"SYS_CHROOT",
			"KILL",
			"AUDIT_WRITE",
		},
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: "NEWNS"},
			{Type: "NEWUTS"},
			{Type: "NEWIPC"},
			{Type: "NEWPID"},
			{Type: "NEWNET"},
		}),
		Cgroups: &configs.Cgroup{
			Parent:          "docker",
			AllowAllDevices: false,
		},
		Mounts: []*configs.Mount{
			{
				Source:      "proc",
				Destination: "/proc",
				Device:      "proc",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "tmpfs",
				Destination: "/dev",
				Device:      "tmpfs",
				Flags:       syscall.MS_NOSUID | syscall.MS_STRICTATIME,
				Data:        "mode=755",
			},
			{
				Source:      "devpts",
				Destination: "/dev/pts",
				Device:      "devpts",
				Flags:       syscall.MS_NOSUID | syscall.MS_NOEXEC,
				Data:        "newinstance,ptmxmode=0666,mode=0620,gid=5",
			},
			{
				Device:      "tmpfs",
				Source:      "shm",
				Destination: "/dev/shm",
				Data:        "mode=1777,size=65536k",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "mqueue",
				Destination: "/dev/mqueue",
				Device:      "mqueue",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "sysfs",
				Destination: "/sys",
				Device:      "sysfs",
				Flags:       defaultMountFlags | syscall.MS_RDONLY,
			},
		},
		MaskPaths: []string{
			"/proc/kcore",
		},
		ReadonlyPaths: []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
	}

	if apparmor.IsEnabled() {
		container.AppArmorProfile = "docker-default"
	}

	return container
}
