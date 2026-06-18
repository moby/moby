//go:build !linux

package daemon

// appArmorSupported returns true if AppArmor is supported and accessible on the host.
func appArmorSupported() bool {
	return false
}
