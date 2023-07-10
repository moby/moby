package types

import "github.com/docker/docker/api/types/system"

// Info contains response of Engine API:
// GET "/info"
//
// Deprecated: use [system.Info].
type Info = system.Info

// Commit holds the Git-commit (SHA1) that a binary was built from, as reported
// in the version-string of external tools, such as containerd, or runC.
//
// Deprecated: use [system.Commit].
type Commit = system.Commit

// PluginsInfo is a temp struct holding Plugins name
// registered with docker daemon. It is used by [system.Info] struct
//
// Deprecated: use [system.PluginsInfo].
type PluginsInfo = system.PluginsInfo

// NetworkAddressPool is a temp struct used by [system.Info] struct.
//
// Deprecated: use [system.NetworkAddressPool].
type NetworkAddressPool = system.NetworkAddressPool

// Runtime describes an OCI runtime.
//
// Deprecated: use [system.Runtime].
type Runtime = system.Runtime

// SecurityOpt contains the name and options of a security option.
//
// Deprecated: use [system.SecurityOpt].
type SecurityOpt = system.SecurityOpt

// KeyValue holds a key/value pair.
//
// Deprecated: use [system.KeyValue].
type KeyValue = system.KeyValue

// DecodeSecurityOptions decodes a security options string slice to a type safe
// [system.SecurityOpt].
//
// Deprecated: use [system.DecodeSecurityOptions].
func DecodeSecurityOptions(opts []string) ([]system.SecurityOpt, error) {
	return system.DecodeSecurityOptions(opts)
}
