package contenthash

import (
	"path/filepath"

	"github.com/Microsoft/go-winio"
)

func (cc *cacheContext) walk(scanPath string, walkFunc filepath.WalkFunc) error {
	// elevating the admin privileges to walk special files/directory
	// like `System Volume Information`, etc. See similar in #4994
	privileges := []string{winio.SeBackupPrivilege}
	return winio.RunWithPrivileges(privileges, func() error {
		return filepath.Walk(scanPath, walkFunc)
	})
}
