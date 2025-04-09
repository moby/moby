//go:build !windows

package contenthash

import "path/filepath"

func (cc *cacheContext) walk(scanPath string, walkFunc filepath.WalkFunc) error {
	return filepath.Walk(scanPath, walkFunc)
}

// This is a no-op on non-Windows
func enableProcessPrivileges() {}

// This is a no-op on non-Windows
func disableProcessPrivileges() {}
