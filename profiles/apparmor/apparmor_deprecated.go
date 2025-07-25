//go:build linux

package apparmor

import "github.com/moby/profiles/apparmor"

// InstallDefault generates a default profile in a temp directory determined by
// os.TempDir(), then loads the profile into the kernel using 'apparmor_parser'.
//
// Deprecated: use [apparmor.InstallDefault].
func InstallDefault(name string) error {
	return apparmor.InstallDefault(name)
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
//
// Deprecated: use [apparmor.IsLoaded].
func IsLoaded(name string) (bool, error) {
	return apparmor.IsLoaded(name)
}
