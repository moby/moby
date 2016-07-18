// +build experimental

package daemon

import (
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/plugin"
	"github.com/docker/engine-api/types/container"
)

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}

func pluginInit(d *Daemon, cfg *Config, remote libcontainerd.Remote) error {
	return plugin.Init(cfg.Root, remote, d.RegistryService, cfg.LiveRestore, d.LogPluginEvent)
}

func pluginShutdown() {
	manager := plugin.GetManager()
	// Check for a valid manager object. In error conditions, daemon init can fail
	// and shutdown called, before plugin manager is initialized.
	if manager != nil {
		manager.Shutdown()
	}
}
