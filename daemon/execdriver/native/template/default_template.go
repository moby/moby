package template

import (
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// New returns the docker default configuration for libcontainer
func New() *libcontainer.Container {
	container := &libcontainer.Container{
		CapabilitiesMask: map[string]bool{
			"SETPCAP":        false,
			"SYS_MODULE":     false,
			"SYS_RAWIO":      false,
			"SYS_PACCT":      false,
			"SYS_ADMIN":      false,
			"SYS_NICE":       false,
			"SYS_RESOURCE":   false,
			"SYS_TIME":       false,
			"SYS_TTY_CONFIG": false,
			"AUDIT_WRITE":    false,
			"AUDIT_CONTROL":  false,
			"MAC_OVERRIDE":   false,
			"MAC_ADMIN":      false,
			"NET_ADMIN":      false,
			"MKNOD":          true,
			"SYSLOG":         false,
			"SETUID":         true,
			"SETGID":         true,
			"CHOWN":          true,
			"NET_RAW":        true,
			"DAC_OVERRIDE":   true,
		},
		Namespaces: map[string]bool{
			"NEWNS":  true,
			"NEWUTS": true,
			"NEWIPC": true,
			"NEWPID": true,
			"NEWNET": true,
		},
		Cgroups: &cgroups.Cgroup{
			Parent:       "docker",
			DeviceAccess: false,
		},
		Context: libcontainer.Context{},
	}
	if apparmor.IsEnabled() {
		container.Context["apparmor_profile"] = "docker-default"
	}
	return container
}
