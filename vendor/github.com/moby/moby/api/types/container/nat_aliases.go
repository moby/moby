package container

import "github.com/docker/go-connections/nat"

// PortProto is a string containing port number and protocol in the format "80/tcp".
// It is the same as [PortRangeProto], but used in places where we only expect
// a single port to be used (not a range).
//
// It is currently an alias for [nat.Port] but may become a concrete type in a future release.
type PortProto = nat.Port

// PortRangeProto is a string containing a range of port numbers and protocol in
// the format "80-90/tcp". It the same as [PortProto], but used in places where
// we expect a port-range to be used.
//
// It is currently an alias for [nat.Port] but may become a concrete type in a future release.
type PortRangeProto = nat.Port

// PortSet is a collection of structs indexed by [PortProto].
type PortSet = map[PortProto]struct{}

// PortBinding represents a binding between a Host IP address and a [HostPort].
//
// It is currently an alias for [nat.PortBinding] but may become a concrete type in a future release.
type PortBinding = nat.PortBinding

// PortMap is a collection of [PortBinding] indexed by [PortProto].
type PortMap = map[PortProto][]PortBinding
