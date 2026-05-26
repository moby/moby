//go:build !linux

package daemon

// AppArmor profile names referenced by shared Unix container setup code.
// AppArmor is only supported on Linux; on other platforms these are
// unused at runtime but must exist for shared code paths to compile.
const (
	unconfinedAppArmorProfile = "unconfined"
	defaultAppArmorProfile    = "docker-default"
)

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
