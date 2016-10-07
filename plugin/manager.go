// +build experimental

package plugin

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/plugin/store"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/registry"
)

var (
	manager *Manager
)

func (pm *Manager) restorePlugin(p *v2.Plugin) error {
	p.RuntimeSourcePath = filepath.Join(pm.runRoot, p.GetID())
	if p.IsEnabled() {
		return pm.restore(p)
	}
	return nil
}

type eventLogger func(id, name, action string)

// Manager controls the plugin subsystem.
type Manager struct {
	libRoot           string
	runRoot           string
	pluginStore       *store.Store
	containerdClient  libcontainerd.Client
	registryService   registry.Service
	liveRestore       bool
	pluginEventLogger eventLogger
}

// GetManager returns the singleton plugin Manager
func GetManager() *Manager {
	return manager
}

// Init (was NewManager) instantiates the singleton Manager.
// TODO: revert this to NewManager once we get rid of all the singletons.
func Init(root string, ps *store.Store, remote libcontainerd.Remote, rs registry.Service, liveRestore bool, evL eventLogger) (err error) {
	if manager != nil {
		return nil
	}

	root = filepath.Join(root, "plugins")
	manager = &Manager{
		libRoot:           root,
		runRoot:           "/run/docker",
		pluginStore:       ps,
		registryService:   rs,
		liveRestore:       liveRestore,
		pluginEventLogger: evL,
	}
	if err := os.MkdirAll(manager.runRoot, 0700); err != nil {
		return err
	}
	manager.containerdClient, err = remote.Client(manager)
	if err != nil {
		return err
	}
	if err := manager.init(); err != nil {
		return err
	}
	return nil
}

// StateChanged updates plugin internals using libcontainerd events.
func (pm *Manager) StateChanged(id string, e libcontainerd.StateInfo) error {
	logrus.Debugf("plugin state changed %s %#v", id, e)

	switch e.State {
	case libcontainerd.StateExit:
		p, err := pm.pluginStore.GetByID(id)
		if err != nil {
			return err
		}
		p.RLock()
		if p.ExitChan != nil {
			close(p.ExitChan)
		}
		restart := p.Restart
		p.RUnlock()
		p.RemoveFromDisk()
		if restart {
			pm.enable(p, true)
		}
	}

	return nil
}

// AttachStreams attaches io streams to the plugin
func (pm *Manager) AttachStreams(id string, iop libcontainerd.IOPipe) error {
	iop.Stdin.Close()

	logger := logrus.New()
	logger.Hooks.Add(logHook{id})
	// TODO: cache writer per id
	w := logger.Writer()
	go func() {
		io.Copy(w, iop.Stdout)
	}()
	go func() {
		// TODO: update logrus and use logger.WriterLevel
		io.Copy(w, iop.Stderr)
	}()
	return nil
}

func (pm *Manager) init() error {
	dt, err := os.Open(filepath.Join(pm.libRoot, "plugins.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer dt.Close()

	plugins := make(map[string]*v2.Plugin)
	if err := json.NewDecoder(dt).Decode(&plugins); err != nil {
		return err
	}
	pm.pluginStore.SetAll(plugins)

	var group sync.WaitGroup
	group.Add(len(plugins))
	for _, p := range plugins {
		go func(p *v2.Plugin) {
			defer group.Done()
			if err := pm.restorePlugin(p); err != nil {
				logrus.Errorf("failed to restore plugin '%s': %s", p.Name(), err)
				return
			}

			pm.pluginStore.Add(p)
			requiresManualRestore := !pm.liveRestore && p.IsEnabled()

			if requiresManualRestore {
				// if liveRestore is not enabled, the plugin will be stopped now so we should enable it
				if err := pm.enable(p, true); err != nil {
					logrus.Errorf("failed to enable plugin '%s': %s", p.Name(), err)
				}
			}
		}(p)
	}
	group.Wait()
	return nil
}

type logHook struct{ id string }

func (logHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (l logHook) Fire(entry *logrus.Entry) error {
	entry.Data = logrus.Fields{"plugin": l.id}
	return nil
}
