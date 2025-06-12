package contenthash

import (
	"path/filepath"
	"sync"

	"github.com/Microsoft/go-winio"
)

var (
	privileges        = []string{winio.SeBackupPrivilege}
	privilegeLock     sync.Mutex
	privilegeRefCount int
)

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
	privilegeLock.Lock()
	defer privilegeLock.Unlock()

	if privilegeRefCount == 0 {
		_ = winio.EnableProcessPrivileges(privileges)
	}
	privilegeRefCount++
}

// Disables the SeBackupPrivilege on the process
// once the group of functions that needed it is complete.
func disableProcessPrivileges() {
	privilegeLock.Lock()
	defer privilegeLock.Unlock()

	if privilegeRefCount > 0 {
		privilegeRefCount--
		if privilegeRefCount == 0 {
			_ = winio.DisableProcessPrivileges(privileges)
		}
	}
}
