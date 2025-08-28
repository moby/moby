package system

import "github.com/moby/moby/api/types/system"

// DiskUsage contains response of Engine API for API 1.49 and greater:
// GET "/system/df"
type DiskUsage = system.DiskUsage

// Info contains response of Engine API:
// GET "/info"
type Info = system.Info

// ContainerdInfo holds information about the containerd instance used by the daemon.
type ContainerdInfo = system.ContainerdInfo

// ContainerdNamespaces reflects the containerd namespaces used by the daemon.
type ContainerdNamespaces = system.ContainerdNamespaces

// PluginsInfo is a temp struct holding Plugins name
// registered with docker daemon. It is used by [Info] struct
type PluginsInfo = system.PluginsInfo

// Commit holds the Git-commit (SHA1) that a binary was built from, as reported
// in the version-string of external tools, such as containerd, or runC.
type Commit = system.Commit

// NetworkAddressPool is a temp struct used by [Info] struct.
type NetworkAddressPool = system.NetworkAddressPool

// FirewallInfo describes the firewall backend.
type FirewallInfo = system.FirewallInfo

// DeviceInfo represents a discoverable device from a device driver.
type DeviceInfo = system.DeviceInfo

// Runtime describes an OCI runtime
type Runtime = system.Runtime

// RuntimeWithStatus extends [Runtime] to hold [RuntimeStatus].
type RuntimeWithStatus = system.RuntimeWithStatus

// SecurityOpt contains the name and options of a security option
type SecurityOpt = system.SecurityOpt

// DecodeSecurityOptions decodes a security options string slice to a
// type-safe [SecurityOpt].
func DecodeSecurityOptions(opts []string) ([]system.SecurityOpt, error) {
	return system.DecodeSecurityOptions(opts)
}

// KeyValue holds a key/value pair.
type KeyValue = system.KeyValue
