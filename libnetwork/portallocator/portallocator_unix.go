//go:build !windows
// +build !windows

package portallocator

const (
	// DefaultPortRangeStart indicates the first port in port range
	DefaultPortRangeStart = 49153
	// DefaultPortRangeEnd indicates the last port in port range
	// consistent with default /proc/sys/net/ipv4/ip_local_port_range
	// upper bound on linux
	DefaultPortRangeEnd = 65535
)
