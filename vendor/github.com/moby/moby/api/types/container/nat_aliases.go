package container

import "github.com/docker/go-connections/nat"

// PortRangeProto is a string containing port number and protocol in the format "80/tcp",
// or a port range and protocol in the format "80-83/tcp".
//
// It is currently an alias for [nat.Port] but may become a concrete type in a future release.
type PortRangeProto = nat.Port

// PortSet is a collection of structs indexed by [HostPort].
//
// It is currently an alias for [nat.PortSet] but may become a concrete type in a future release.
type PortSet = nat.PortSet

// PortBinding represents a binding between a Host IP address and a [HostPort].
//
// It is currently an alias for [nat.PortBinding] but may become a concrete type in a future release.
type PortBinding = nat.PortBinding

// PortMap is a collection of [PortBinding] indexed by [HostPort].
//
// It is currently an alias for [nat.PortMap] but may become a concrete type in a future release.
type PortMap = nat.PortMap
