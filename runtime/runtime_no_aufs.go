// +build exclude_graphdriver_aufs

package runtime

import (
	"github.com/dotcloud/docker/runtime/graphdriver"
)

func migrateIfAufs(driver graphdriver.Driver, root string) error {
	return nil
}
