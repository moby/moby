//go:build !linux

package daemon

func loadDefaultAppArmorProfileIfMissing() error {
	return nil
}

// DefaultApparmorProfile returns an empty string.
func DefaultApparmorProfile() string {
	return ""
}

func installDefaultAppArmorProfile() error {
	return nil
}
