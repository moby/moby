// +build linux

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/initlayer"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/plugin/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func (pm *Manager) enable(p *v2.Plugin, c *controller, force bool) error {
	p.Rootfs = filepath.Join(pm.config.Root, p.PluginObj.ID, "rootfs")
	if p.IsEnabled() && !force {
		return fmt.Errorf("plugin %s is already enabled", p.Name())
	}
	spec, err := p.InitSpec(pm.config.ExecRoot)
	if err != nil {
		return err
	}

	c.restart = true
	c.exitChan = make(chan bool)

	pm.mu.Lock()
	pm.cMap[p] = c
	pm.mu.Unlock()

	if p.PropagatedMount != "" {
		if err := mount.MakeRShared(p.PropagatedMount); err != nil {
			return err
		}
	}

	if err := initlayer.Setup(filepath.Join(pm.config.Root, p.PluginObj.ID, rootFSFileName), 0, 0); err != nil {
		return err
	}

	if err := pm.containerdClient.Create(p.GetID(), "", "", specs.Spec(*spec), attachToLog(p.GetID())); err != nil {
		if p.PropagatedMount != "" {
			if err := mount.Unmount(p.PropagatedMount); err != nil {
				logrus.Warnf("Could not unmount %s: %v", p.PropagatedMount, err)
			}
		}
		return err
	}

	return pm.pluginPostStart(p, c)
}

func (pm *Manager) pluginPostStart(p *v2.Plugin, c *controller) error {
	client, err := plugins.NewClientWithTimeout("unix://"+filepath.Join(pm.config.ExecRoot, p.GetID(), p.GetSocket()), nil, c.timeoutInSecs)
	if err != nil {
		c.restart = false
		shutdownPlugin(p, c, pm.containerdClient)
		return err
	}

	p.SetPClient(client)
	pm.config.Store.SetState(p, true)
	pm.config.Store.CallHandler(p)

	return pm.save(p)
}

func (pm *Manager) restore(p *v2.Plugin) error {
	if err := pm.containerdClient.Restore(p.GetID(), attachToLog(p.GetID())); err != nil {
		return err
	}

	if pm.config.LiveRestoreEnabled {
		c := &controller{}
		if pids, _ := pm.containerdClient.GetPidsForContainer(p.GetID()); len(pids) == 0 {
			// plugin is not running, so follow normal startup procedure
			return pm.enable(p, c, true)
		}

		c.exitChan = make(chan bool)
		c.restart = true
		pm.mu.Lock()
		pm.cMap[p] = c
		pm.mu.Unlock()
		return pm.pluginPostStart(p, c)
	}

	return nil
}

func shutdownPlugin(p *v2.Plugin, c *controller, containerdClient libcontainerd.Client) {
	pluginID := p.GetID()

	err := containerdClient.Signal(pluginID, int(syscall.SIGTERM))
	if err != nil {
		logrus.Errorf("Sending SIGTERM to plugin failed with error: %v", err)
	} else {
		select {
		case <-c.exitChan:
			logrus.Debug("Clean shutdown of plugin")
		case <-time.After(time.Second * 10):
			logrus.Debug("Force shutdown plugin")
			if err := containerdClient.Signal(pluginID, int(syscall.SIGKILL)); err != nil {
				logrus.Errorf("Sending SIGKILL to plugin failed with error: %v", err)
			}
		}
	}
}

func (pm *Manager) disable(p *v2.Plugin, c *controller) error {
	if !p.IsEnabled() {
		return fmt.Errorf("plugin %s is already disabled", p.Name())
	}

	c.restart = false
	shutdownPlugin(p, c, pm.containerdClient)
	pm.config.Store.SetState(p, false)
	return pm.save(p)
}

// Shutdown stops all plugins and called during daemon shutdown.
func (pm *Manager) Shutdown() {
	plugins := pm.config.Store.GetAll()
	for _, p := range plugins {
		pm.mu.RLock()
		c := pm.cMap[p]
		pm.mu.RUnlock()

		if pm.config.LiveRestoreEnabled && p.IsEnabled() {
			logrus.Debug("Plugin active when liveRestore is set, skipping shutdown")
			continue
		}
		if pm.containerdClient != nil && p.IsEnabled() {
			c.restart = false
			shutdownPlugin(p, c, pm.containerdClient)
		}
	}
}

// createPlugin creates a new plugin. take lock before calling.
func (pm *Manager) createPlugin(name string, configDigest digest.Digest, blobsums []digest.Digest, rootFSDir string, privileges *types.PluginPrivileges) (p *v2.Plugin, err error) {
	if err := pm.config.Store.validateName(name); err != nil { // todo: this check is wrong. remove store
		return nil, err
	}

	configRC, err := pm.blobStore.Get(configDigest)
	if err != nil {
		return nil, err
	}
	defer configRC.Close()

	var config types.PluginConfig
	dec := json.NewDecoder(configRC)
	if err := dec.Decode(&config); err != nil {
		return nil, errors.Wrapf(err, "failed to parse config")
	}
	if dec.More() {
		return nil, errors.New("invalid config json")
	}

	requiredPrivileges, err := computePrivileges(config)
	if err != nil {
		return nil, err
	}
	if privileges != nil {
		if err := validatePrivileges(requiredPrivileges, *privileges); err != nil {
			return nil, err
		}
	}

	p = &v2.Plugin{
		PluginObj: types.Plugin{
			Name:   name,
			ID:     stringid.GenerateRandomID(),
			Config: config,
		},
		Config:   configDigest,
		Blobsums: blobsums,
	}
	p.InitEmptySettings()

	pdir := filepath.Join(pm.config.Root, p.PluginObj.ID)
	if err := os.MkdirAll(pdir, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %v", pdir)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(pdir)
		}
	}()

	if err := os.Rename(rootFSDir, filepath.Join(pdir, rootFSFileName)); err != nil {
		return nil, errors.Wrap(err, "failed to rename rootfs")
	}

	if err := pm.save(p); err != nil {
		return nil, err
	}

	pm.config.Store.Add(p) // todo: remove

	return p, nil
}
