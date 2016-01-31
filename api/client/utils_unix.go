// +build !windows

package client

import (
	"path/filepath"
)

func getContextRoot(srcPath string) (string, error) {
	return filepath.Join(srcPath, "."), nil
}
