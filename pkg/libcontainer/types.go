package libcontainer

// These constants are defined as string types so that
// it is clear when adding the configuration in config files
// instead of using ints or other types
const (
	CAP_SETPCAP        Capability = "SETPCAP"
	CAP_SYS_MODULE     Capability = "SYS_MODULE"
	CAP_SYS_RAWIO      Capability = "SYS_RAWIO"
	CAP_SYS_PACCT      Capability = "SYS_PACCT"
	CAP_SYS_ADMIN      Capability = "SYS_ADMIN"
	CAP_SYS_NICE       Capability = "SYS_NICE"
	CAP_SYS_RESOURCE   Capability = "SYS_RESOURCE"
	CAP_SYS_TIME       Capability = "SYS_TIME"
	CAP_SYS_TTY_CONFIG Capability = "SYS_TTY_CONFIG"
	CAP_MKNOD          Capability = "MKNOD"
	CAP_AUDIT_WRITE    Capability = "AUDIT_WRITE"
	CAP_AUDIT_CONTROL  Capability = "AUDIT_CONTROL"
	CAP_MAC_OVERRIDE   Capability = "MAC_OVERRIDE"
	CAP_MAC_ADMIN      Capability = "MAC_ADMIN"
	CAP_NET_ADMIN      Capability = "NET_ADMIN"

	CLONE_NEWNS   Namespace = "NEWNS"   // mount
	CLONE_NEWUTS  Namespace = "NEWUTS"  // utsname
	CLONE_NEWIPC  Namespace = "NEWIPC"  // ipc
	CLONE_NEWUSER Namespace = "NEWUSER" // user
	CLONE_NEWPID  Namespace = "NEWPID"  // pid
	CLONE_NEWNET  Namespace = "NEWNET"  // network
)

type (
	Namespace    string
	Namespaces   []Namespace
	Capability   string
	Capabilities []Capability
)

// Contains returns true if the specified Namespace is
// in the slice
func (n Namespaces) Contains(ns Namespace) bool {
	for _, nns := range n {
		if nns == ns {
			return true
		}
	}
	return false
}

// Contains returns true if the specified Capability is
// in the slice
func (c Capabilities) Contains(capp Capability) bool {
	for _, cc := range c {
		if cc == capp {
			return true
		}
	}
	return false
}
