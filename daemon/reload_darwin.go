package daemon // import "github.com/docker/docker/daemon"

import "github.com/docker/docker/daemon/config"

func (daemon *Daemon) reloadPlatform(txn *reloadTxn, newCfg *configStore, conf *config.Config, attributes map[string]string) error {
	return nil
}
