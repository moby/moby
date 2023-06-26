package homedir

import (
	"os"
	"os/user"
	"runtime"
)

// Key returns the env var name for the user's home dir based on
// the platform being run on.
func Key() string {
	return envKeyName
}

// Get returns the home directory of the current user with the help of
// environment variables depending on the target operating system.
// Returned path should be used with "path/filepath" to form new paths.
//
// On non-Windows platforms, it falls back to nss lookups, if the home
// directory cannot be obtained from environment-variables.
//
// If linking statically with cgo enabled against glibc, ensure the
// osusergo build tag is used.
//
// If needing to do nss lookups, do not disable cgo or set osusergo.
func Get() string {
	home, _ := os.UserHomeDir()
	if home == "" && runtime.GOOS != "windows" {
		if u, err := user.Current(); err == nil {
			return u.HomeDir
		}
	}
	return home
}

// GetShortcutString returns the string that is shortcut to user's home directory
// in the native shell of the platform running on.
func GetShortcutString() string {
	return homeShortCut
}
