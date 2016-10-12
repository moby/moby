// +build experimental

package daemon

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/plugin"
)

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}

func pluginInit(d *Daemon, cfg *Config, remote libcontainerd.Remote) error {
	return plugin.Init(cfg.Root, d.PluginStore, remote, d.RegistryService, cfg.LiveRestoreEnabled, d.LogPluginEvent)
}

func pluginShutdown() {
	manager := plugin.GetManager()
	// Check for a valid manager object. In error conditions, daemon init can fail
	// and shutdown called, before plugin manager is initialized.
	if manager != nil {
		manager.Shutdown()
	}
}
