//go:build !linux

package daemon // import "github.com/docker/docker/daemon"

func clobberDefaultAppArmorProfile() error {
	return nil
}

func ensureDefaultAppArmorProfile() error {
	return nil
}

// DefaultAppArmorProfile returns an empty string.
func DefaultAppArmorProfile() string {
	return ""
}
