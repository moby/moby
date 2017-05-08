// +build !windows

package builder

import (
	"path/filepath"
)

func getContextRoot(srcPath string) (string, error) {
	return filepath.Join(srcPath, "."), nil
}
