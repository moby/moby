// +build linux

package libcontainer

import (
	"fmt"
	"os"

	"github.com/syndtr/gocapability/capability"
)

const allCapabilityTypes = capability.CAPS | capability.BOUNDS

var capabilityList = map[string]capability.Cap{
	"CAP_SETPCAP":          capability.CAP_SETPCAP,
	"CAP_SYS_MODULE":       capability.CAP_SYS_MODULE,
	"CAP_SYS_RAWIO":        capability.CAP_SYS_RAWIO,
	"CAP_SYS_PACCT":        capability.CAP_SYS_PACCT,
	"CAP_SYS_ADMIN":        capability.CAP_SYS_ADMIN,
	"CAP_SYS_NICE":         capability.CAP_SYS_NICE,
	"CAP_SYS_RESOURCE":     capability.CAP_SYS_RESOURCE,
	"CAP_SYS_TIME":         capability.CAP_SYS_TIME,
	"CAP_SYS_TTY_CONFIG":   capability.CAP_SYS_TTY_CONFIG,
	"CAP_MKNOD":            capability.CAP_MKNOD,
	"CAP_AUDIT_WRITE":      capability.CAP_AUDIT_WRITE,
	"CAP_AUDIT_CONTROL":    capability.CAP_AUDIT_CONTROL,
	"CAP_MAC_OVERRIDE":     capability.CAP_MAC_OVERRIDE,
	"CAP_MAC_ADMIN":        capability.CAP_MAC_ADMIN,
	"CAP_NET_ADMIN":        capability.CAP_NET_ADMIN,
	"CAP_SYSLOG":           capability.CAP_SYSLOG,
	"CAP_CHOWN":            capability.CAP_CHOWN,
	"CAP_NET_RAW":          capability.CAP_NET_RAW,
	"CAP_DAC_OVERRIDE":     capability.CAP_DAC_OVERRIDE,
	"CAP_FOWNER":           capability.CAP_FOWNER,
	"CAP_DAC_READ_SEARCH":  capability.CAP_DAC_READ_SEARCH,
	"CAP_FSETID":           capability.CAP_FSETID,
	"CAP_KILL":             capability.CAP_KILL,
	"CAP_SETGID":           capability.CAP_SETGID,
	"CAP_SETUID":           capability.CAP_SETUID,
	"CAP_LINUX_IMMUTABLE":  capability.CAP_LINUX_IMMUTABLE,
	"CAP_NET_BIND_SERVICE": capability.CAP_NET_BIND_SERVICE,
	"CAP_NET_BROADCAST":    capability.CAP_NET_BROADCAST,
	"CAP_IPC_LOCK":         capability.CAP_IPC_LOCK,
	"CAP_IPC_OWNER":        capability.CAP_IPC_OWNER,
	"CAP_SYS_CHROOT":       capability.CAP_SYS_CHROOT,
	"CAP_SYS_PTRACE":       capability.CAP_SYS_PTRACE,
	"CAP_SYS_BOOT":         capability.CAP_SYS_BOOT,
	"CAP_LEASE":            capability.CAP_LEASE,
	"CAP_SETFCAP":          capability.CAP_SETFCAP,
	"CAP_WAKE_ALARM":       capability.CAP_WAKE_ALARM,
	"CAP_BLOCK_SUSPEND":    capability.CAP_BLOCK_SUSPEND,
	"CAP_AUDIT_READ":       capability.CAP_AUDIT_READ,
}

func newCapWhitelist(caps []string) (*whitelist, error) {
	l := []capability.Cap{}
	for _, c := range caps {
		v, ok := capabilityList[c]
		if !ok {
			return nil, fmt.Errorf("unknown capability %q", c)
		}
		l = append(l, v)
	}
	pid, err := capability.NewPid(os.Getpid())
	if err != nil {
		return nil, err
	}
	return &whitelist{
		keep: l,
		pid:  pid,
	}, nil
}

type whitelist struct {
	pid  capability.Capabilities
	keep []capability.Cap
}

// dropBoundingSet drops the capability bounding set to those specified in the whitelist.
func (w *whitelist) dropBoundingSet() error {
	w.pid.Clear(capability.BOUNDS)
	w.pid.Set(capability.BOUNDS, w.keep...)
	return w.pid.Apply(capability.BOUNDS)
}

// drop drops all capabilities for the current process except those specified in the whitelist.
func (w *whitelist) drop() error {
	w.pid.Clear(allCapabilityTypes)
	w.pid.Set(allCapabilityTypes, w.keep...)
	return w.pid.Apply(allCapabilityTypes)
}
