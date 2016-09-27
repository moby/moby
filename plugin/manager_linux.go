// +build linux,experimental

package plugin

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/restartmanager"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (pm *Manager) enable(p *v2.Plugin, force bool) error {
	if p.IsEnabled() && !force {
		return fmt.Errorf("plugin %s is already enabled", p.Name())
	}
	spec, err := p.InitSpec(oci.DefaultSpec(), pm.libRoot)
	if err != nil {
		return err
	}

	p.RestartManager = restartmanager.New(container.RestartPolicy{Name: "always"}, 0)
	if err := pm.containerdClient.Create(p.GetID(), "", "", specs.Spec(*spec), libcontainerd.WithRestartManager(p.RestartManager)); err != nil {
		if err := p.RestartManager.Cancel(); err != nil {
			logrus.Errorf("enable: restartManager.Cancel failed due to %v", err)
		}
		return err
	}

	p.PClient, err = plugins.NewClient("unix://"+filepath.Join(p.RuntimeSourcePath, p.GetSocket()), nil)
	if err != nil {
		if err := p.RestartManager.Cancel(); err != nil {
			logrus.Errorf("enable: restartManager.Cancel failed due to %v", err)
		}
		return err
	}

	pm.pluginStore.SetState(p, true)
	pm.pluginStore.CallHandler(p)

	return nil
}

func (pm *Manager) restore(p *v2.Plugin) error {
	p.RestartManager = restartmanager.New(container.RestartPolicy{Name: "always"}, 0)
	return pm.containerdClient.Restore(p.GetID(), libcontainerd.WithRestartManager(p.RestartManager))
}

func (pm *Manager) disable(p *v2.Plugin) error {
	if !p.IsEnabled() {
		return fmt.Errorf("plugin %s is already disabled", p.Name())
	}
	if err := p.RestartManager.Cancel(); err != nil {
		logrus.Error(err)
	}
	if err := pm.containerdClient.Signal(p.GetID(), int(syscall.SIGKILL)); err != nil {
		logrus.Error(err)
	}
	if err := p.RemoveFromDisk(); err != nil {
		logrus.Error(err)
	}
	pm.pluginStore.SetState(p, false)
	return nil
}

// Shutdown stops all plugins and called during daemon shutdown.
func (pm *Manager) Shutdown() {
	pm.Lock()
	pm.shutdown = true
	pm.Unlock()

	pm.RLock()
	defer pm.RUnlock()
	plugins := pm.pluginStore.GetAll()
	for _, p := range plugins {
		if pm.liveRestore && p.IsEnabled() {
			logrus.Debug("Plugin active when liveRestore is set, skipping shutdown")
			continue
		}
		if p.RestartManager != nil {
			if err := p.RestartManager.Cancel(); err != nil {
				logrus.Error(err)
			}
		}
		if pm.containerdClient != nil && p.IsEnabled() {
			pluginID := p.GetID()
			p.ExitChan = make(chan bool)
			err := pm.containerdClient.Signal(p.PluginObj.ID, int(syscall.SIGTERM))
			if err != nil {
				logrus.Errorf("Sending SIGTERM to plugin failed with error: %v", err)
			} else {
				select {
				case <-p.ExitChan:
					logrus.Debug("Clean shutdown of plugin")
				case <-time.After(time.Second * 10):
					logrus.Debug("Force shutdown plugin")
					if err := pm.containerdClient.Signal(pluginID, int(syscall.SIGKILL)); err != nil {
						logrus.Errorf("Sending SIGKILL to plugin failed with error: %v", err)
					}
				}
			}
		}
		if err := p.RemoveFromDisk(); err != nil {
			logrus.Errorf("Remove plugin runtime failed with error: %v", err)
		}
	}
}
