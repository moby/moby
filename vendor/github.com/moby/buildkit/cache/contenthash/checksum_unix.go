//go:build !windows
// +build !windows

package contenthash

import "path/filepath"

func (cc *cacheContext) walk(scanPath string, walkFunc filepath.WalkFunc) error {
	return filepath.Walk(scanPath, walkFunc)
}
