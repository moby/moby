//go:build !linux

package daemon

func ensureDefaultAppArmorProfile() error {
	return nil
}

// DefaultApparmorProfile returns an empty string.
func DefaultApparmorProfile() string {
	return ""
}
