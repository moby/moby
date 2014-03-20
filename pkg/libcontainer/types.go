package libcontainer

import (
	"errors"
	"github.com/syndtr/gocapability/capability"
)

var (
	ErrUnkownNamespace  = errors.New("Unknown namespace")
	ErrUnkownCapability = errors.New("Unknown capability")
	ErrUnsupported      = errors.New("Unsupported method")
)

// namespaceList is used to convert the libcontainer types
// into the names of the files located in /proc/<pid>/ns/* for
// each namespace
var (
	namespaceList = Namespaces{}

	capabilityList = Capabilities{
		{Key: "SETPCAP", Value: capability.CAP_SETPCAP, Enabled: true},
		{Key: "SYS_MODULE", Value: capability.CAP_SYS_MODULE, Enabled: true},
		{Key: "SYS_RAWIO", Value: capability.CAP_SYS_RAWIO, Enabled: true},
		{Key: "SYS_PACCT", Value: capability.CAP_SYS_PACCT, Enabled: true},
		{Key: "SYS_ADMIN", Value: capability.CAP_SYS_ADMIN, Enabled: true},
		{Key: "SYS_NICE", Value: capability.CAP_SYS_NICE, Enabled: true},
		{Key: "SYS_RESOURCE", Value: capability.CAP_SYS_RESOURCE, Enabled: true},
		{Key: "SYS_TIME", Value: capability.CAP_SYS_TIME, Enabled: true},
		{Key: "SYS_TTY_CONFIG", Value: capability.CAP_SYS_TTY_CONFIG, Enabled: true},
		{Key: "MKNOD", Value: capability.CAP_MKNOD, Enabled: true},
		{Key: "AUDIT_WRITE", Value: capability.CAP_AUDIT_WRITE, Enabled: true},
		{Key: "AUDIT_CONTROL", Value: capability.CAP_AUDIT_CONTROL, Enabled: true},
		{Key: "MAC_OVERRIDE", Value: capability.CAP_MAC_OVERRIDE, Enabled: true},
		{Key: "MAC_ADMIN", Value: capability.CAP_MAC_ADMIN, Enabled: true},
		{Key: "NET_ADMIN", Value: capability.CAP_NET_ADMIN, Enabled: true},
	}
)

type (
	Namespace struct {
		Key     string `json:"key,omitempty"`
		Enabled bool   `json:"enabled,omitempty"`
		Value   int    `json:"value,omitempty"`
		File    string `json:"file,omitempty"`
	}
	Namespaces []*Namespace
)

func (ns *Namespace) String() string {
	return ns.Key
}

func GetNamespace(key string) *Namespace {
	for _, ns := range namespaceList {
		if ns.Key == key {
			return ns
		}
	}
	return nil
}

// Contains returns true if the specified Namespace is
// in the slice
func (n Namespaces) Contains(ns string) bool {
	for _, nsp := range n {
		if nsp.Key == ns {
			return true
		}
	}
	return false
}

type (
	Capability struct {
		Key     string         `json:"key,omitempty"`
		Enabled bool           `json:"enabled"`
		Value   capability.Cap `json:"value,omitempty"`
	}
	Capabilities []*Capability
)

func (c *Capability) String() string {
	return c.Key
}

func GetCapability(key string) *Capability {
	for _, capp := range capabilityList {
		if capp.Key == key {
			return capp
		}
	}
	return nil
}

// Contains returns true if the specified Capability is
// in the slice
func (c Capabilities) Contains(capp string) bool {
	for _, cap := range c {
		if cap.Key == capp {
			return true
		}
	}
	return false
}
