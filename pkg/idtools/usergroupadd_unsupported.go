// +build !linux

package idtools

import (
	"fmt"
	"os"

	"github.com/docker/docker/pkg/system"
)

// AddNamespaceRangesUser takes a name and finds an unused uid, gid pair
// and calls the appropriate helper function to add the group and then
// the user to the group in /etc/group and /etc/passwd respectively.
func AddNamespaceRangesUser(name string) (int, int, error) {
	return -1, -1, fmt.Errorf("No support for adding users or groups on this OS")
}

// Platforms such as Windows do not support the UID/GID concept. So make this
// just a wrapper around system.MkdirAll.
func mkdirAs(path string, mode os.FileMode, ownerUID, ownerGID int, mkAll bool) error {
	if err := system.MkdirAll(path, mode); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}
