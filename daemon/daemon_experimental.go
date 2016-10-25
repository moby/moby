package daemon

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/plugin"
)

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}

func (daemon *Daemon) pluginInit(cfg *Config, remote libcontainerd.Remote) error {
	if !daemon.HasExperimental() {
		return nil
	}
	return plugin.Init(cfg.Root, daemon.PluginStore, remote, daemon.RegistryService, cfg.LiveRestoreEnabled, daemon.LogPluginEvent)
}

func (daemon *Daemon) pluginShutdown() {
	if !daemon.HasExperimental() {
		return
	}
	manager := plugin.GetManager()
	// Check for a valid manager object. In error conditions, daemon init can fail
	// and shutdown called, before plugin manager is initialized.
	if manager != nil {
		manager.Shutdown()
	}
}
