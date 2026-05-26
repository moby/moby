//go:build !linux && !freebsd && !windows

package portallocator

// getDynamicPortRange returns the package defaults on platforms without a
// real port-range discovery implementation. The daemon does not run on
// these platforms, so the actual values are unused at runtime.
func getDynamicPortRange() (start int, end int, _ error) {
	return defaultPortRangeStart, defaultPortRangeEnd, nil
}
