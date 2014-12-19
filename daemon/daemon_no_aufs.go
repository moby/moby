// +build exclude_graphdriver_aufs

package daemon

import (
	"github.com/docker/docker/storage"
)

func migrateIfAufs(driver storage.Driver, root string) error {
	return nil
}
