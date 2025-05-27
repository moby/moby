package contenthash

import (
	"path/filepath"

	"github.com/Microsoft/go-winio"
)

var privileges = []string{winio.SeBackupPrivilege}

func (cc *cacheContext) walk(scanPath string, walkFunc filepath.WalkFunc) error {
	// elevating the admin privileges to walk special files/directory
	// like `System Volume Information`, etc. See similar in #4994
	return winio.RunWithPrivileges(privileges, func() error {
		return filepath.Walk(scanPath, walkFunc)
	})
}

// Adds the SeBackupPrivilege to the process
// to be able to access some special files and directories.
func enableProcessPrivileges() {
	_ = winio.EnableProcessPrivileges(privileges)
}

// Disables the SeBackupPrivilege on the process
// once the group of functions that needed it is complete.
func disableProcessPrivileges() {
	_ = winio.DisableProcessPrivileges(privileges)
}
