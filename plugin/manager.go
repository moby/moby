package plugin

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/plugin/store"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/registry"
)

var (
	manager *Manager
)

func (pm *Manager) restorePlugin(p *v2.Plugin) error {
	p.Restore(pm.runRoot)
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
	mu                sync.RWMutex // protects cMap
	cMap              map[*v2.Plugin]*controller
}

// controller represents the manager's control on a plugin.
type controller struct {
	restart       bool
	exitChan      chan bool
	timeoutInSecs int
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
		runRoot:           "/run/docker/plugins",
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
	manager.cMap = make(map[*v2.Plugin]*controller)
	return manager.reload()
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

		pm.mu.RLock()
		c := pm.cMap[p]

		if c.exitChan != nil {
			close(c.exitChan)
		}
		restart := c.restart
		pm.mu.RUnlock()

		p.RemoveFromDisk()

		if p.PropagatedMount != "" {
			if err := mount.Unmount(p.PropagatedMount); err != nil {
				logrus.Warnf("Could not unmount %s: %v", p.PropagatedMount, err)
			}
		}

		if restart {
			pm.enable(p, c, true)
		}
	}

	return nil
}

// reload is used on daemon restarts to load the manager's state
func (pm *Manager) reload() error {
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
		c := &controller{}
		pm.cMap[p] = c
		go func(p *v2.Plugin) {
			defer group.Done()
			if err := pm.restorePlugin(p); err != nil {
				logrus.Errorf("failed to restore plugin '%s': %s", p.Name(), err)
				return
			}

			if p.Rootfs != "" {
				p.Rootfs = filepath.Join(pm.libRoot, p.PluginObj.ID, "rootfs")
			}

			// We should only enable rootfs propagation for certain plugin types that need it.
			for _, typ := range p.PluginObj.Config.Interface.Types {
				if (typ.Capability == "volumedriver" || typ.Capability == "graphdriver") && typ.Prefix == "docker" && strings.HasPrefix(typ.Version, "1.") {
					if p.PluginObj.Config.PropagatedMount != "" {
						// TODO: sanitize PropagatedMount and prevent breakout
						p.PropagatedMount = filepath.Join(p.Rootfs, p.PluginObj.Config.PropagatedMount)
						if err := os.MkdirAll(p.PropagatedMount, 0755); err != nil {
							logrus.Errorf("failed to create PropagatedMount directory at %s: %v", p.PropagatedMount, err)
							return
						}
					}
				}
			}

			pm.pluginStore.Update(p)
			requiresManualRestore := !pm.liveRestore && p.IsEnabled()

			if requiresManualRestore {
				// if liveRestore is not enabled, the plugin will be stopped now so we should enable it
				if err := pm.enable(p, c, true); err != nil {
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

func attachToLog(id string) func(libcontainerd.IOPipe) error {
	return func(iop libcontainerd.IOPipe) error {
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
}
