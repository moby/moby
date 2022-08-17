package daemon // import "github.com/docker/docker/daemon"

import "github.com/docker/docker/daemon/config"

// reloadPlatform updates configuration with platform specific options
// and updates the passed attributes
func (daemon *Daemon) reloadPlatform(txn *reloadTxn, newCfg, conf *config.Config, attributes map[string]string) error {
	return nil
}
