package platform

import (
	"errors"
	"runtime"
	"strings"
)

const (
	Linux   = "linux"
	Windows = "windows"
)

const (
	IsLinux   = runtime.GOOS == "linux"
	IsWindows = runtime.GOOS == "windows"
)

// SelectVersion extracts the platform-appropriate version from a list of strings like
// `linux:6.1` or `windows:0.20.0`.
//
// Returns an empty string and nil if no version matched or an error if no strings were passed.
func SelectVersion(versions []string) (string, error) {
	const prefix = runtime.GOOS + ":"

	if len(versions) == 0 {
		return "", errors.New("no versions specified")
	}

	for _, version := range versions {
		if after, ok := strings.CutPrefix(version, prefix); ok {
			return after, nil
		}

		if IsLinux && !strings.ContainsRune(version, ':') {
			// Allow version numbers without a GOOS prefix on Linux.
			return version, nil
		}
	}

	return "", nil
}
