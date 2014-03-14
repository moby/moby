// +build !exclude_graphdriver_aufs

package runtime

import (
	"github.com/dotcloud/docker/graph"
	"github.com/dotcloud/docker/runtime/graphdriver"
	"github.com/dotcloud/docker/runtime/graphdriver/aufs"
	"github.com/dotcloud/docker/utils"
)

// Given the graphdriver ad, if it is aufs, then migrate it.
// If aufs driver is not built, this func is a noop.
func migrateIfAufs(driver graphdriver.Driver, root string) error {
	if ad, ok := driver.(*aufs.Driver); ok {
		utils.Debugf("Migrating existing containers")
		if err := ad.Migrate(root, graph.SetupInitLayer); err != nil {
			return err
		}
	}
	return nil
}
