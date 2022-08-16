//go:build !windows
// +build !windows

package portallocator

const (
	// defaultPortRangeStart indicates the first port in port range
	defaultPortRangeStart = 49153
	// defaultPortRangeEnd indicates the last port in port range
	// consistent with default /proc/sys/net/ipv4/ip_local_port_range
	// upper bound on linux
	defaultPortRangeEnd = 65535
)
