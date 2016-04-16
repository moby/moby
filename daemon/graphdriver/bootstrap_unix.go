// +build !windows

package graphdriver

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/idtools"
)

// setupDaemonRoot initializes the root directory with the right mode.
func setupDaemonRoot(rootDir string, rootUID, rootGID int) error {
	// the docker root metadata directory needs to have execute permissions for all users (o+x)
	// so that syscalls executing as non-root, operating on subdirectories of the graph root
	// (e.g. mounted layers of a container) can traverse this path.
	// The user namespace support will create subdirectories for the remapped root host uid:gid
	// pair owned by that same uid:gid pair for proper write access to those needed metadata and
	// layer content subtrees.
	if _, err := os.Stat(rootDir); err == nil {
		// root current exists; verify the access bits are correct by setting them
		if rootUID == 0 && rootGID == 0 {
			if err = os.Chmod(rootDir, 0711); err != nil {
				return err
			}
		}
	} else if os.IsNotExist(err) {
		// if user namespaces are enabled we will create a subtree underneath the specified root
		// with any/all specified remapped root uid/gid options on the daemon creating
		// a new subdirectory with ownership set to the remapped uid/gid (so as to allow
		// `chdir()` to work for containers namespaced to that uid/gid)
		if rootUID != 0 && rootGID != 0 {
			logrus.Debugf("Creating user namespaced daemon root: %s", rootDir)
			// Create the root directory if it doesn't exists
			if err := idtools.MkdirAllAs(rootDir, 0700, rootUID, rootGID); err != nil {
				return fmt.Errorf("Cannot create daemon root: %s: %v", rootDir, err)
			}
		} else {
			// no root exists yet, create it 0701 with root:root ownership
			if err := os.MkdirAll(rootDir, 0711); err != nil {
				return err
			}
		}
	}
	return nil
}
