package homedir

import (
	"os"
	"runtime"
)

// Get returns the home directory of the current user with the help of
// environment variables depending on the target operating system.
// Returned path should be used with "path/filepath" to form new paths.
func Get() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

// GetShortcutString returns the string that is shortcut to user's home directory
// in the native shell of the platform running on.
func GetShortcutString() string {
	if runtime.GOOS == "windows" {
		return "%USERPROFILE%" // be careful while using in format functions
	}
	return "~"
}
