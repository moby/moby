package plugin

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const configFileName = "config.json"
const rootFSFileName = "rootfs"

var validFullID = regexp.MustCompile(`^([a-f0-9]{64})$`)

func (pm *Manager) restorePlugin(p *v2.Plugin) error {
	if p.IsEnabled() {
		return pm.restore(p)
	}
	return nil
}

type eventLogger func(id, name, action string)

// ManagerConfig defines configuration needed to start new manager.
type ManagerConfig struct {
	Store              *Store // remove
	Executor           libcontainerd.Remote
	RegistryService    registry.Service
	LiveRestoreEnabled bool // TODO: remove
	LogPluginEvent     eventLogger
	Root               string
	ExecRoot           string
}

// Manager controls the plugin subsystem.
type Manager struct {
	config           ManagerConfig
	mu               sync.RWMutex // protects cMap
	muGC             sync.RWMutex // protects blobstore deletions
	cMap             map[*v2.Plugin]*controller
	containerdClient libcontainerd.Client
	blobStore        *basicBlobStore
}

// controller represents the manager's control on a plugin.
type controller struct {
	restart       bool
	exitChan      chan bool
	timeoutInSecs int
}

// pluginRegistryService ensures that all resolved repositories
// are of the plugin class.
type pluginRegistryService struct {
	registry.Service
}

func (s pluginRegistryService) ResolveRepository(name reference.Named) (repoInfo *registry.RepositoryInfo, err error) {
	repoInfo, err = s.Service.ResolveRepository(name)
	if repoInfo != nil {
		repoInfo.Class = "plugin"
	}
	return
}

// NewManager returns a new plugin manager.
func NewManager(config ManagerConfig) (*Manager, error) {
	if config.RegistryService != nil {
		config.RegistryService = pluginRegistryService{config.RegistryService}
	}
	manager := &Manager{
		config: config,
	}
	if err := os.MkdirAll(manager.config.Root, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %v", manager.config.Root)
	}
	if err := os.MkdirAll(manager.config.ExecRoot, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %v", manager.config.ExecRoot)
	}
	if err := os.MkdirAll(manager.tmpDir(), 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %v", manager.tmpDir())
	}
	var err error
	manager.containerdClient, err = config.Executor.Client(manager) // todo: move to another struct
	if err != nil {
		return nil, errors.Wrap(err, "failed to create containerd client")
	}
	manager.blobStore, err = newBasicBlobStore(filepath.Join(manager.config.Root, "storage/blobs"))
	if err != nil {
		return nil, err
	}

	manager.cMap = make(map[*v2.Plugin]*controller)
	if err := manager.reload(); err != nil {
		return nil, errors.Wrap(err, "failed to restore plugins")
	}
	return manager, nil
}

func (pm *Manager) tmpDir() string {
	return filepath.Join(pm.config.Root, "tmp")
}

// StateChanged updates plugin internals using libcontainerd events.
func (pm *Manager) StateChanged(id string, e libcontainerd.StateInfo) error {
	logrus.Debugf("plugin state changed %s %#v", id, e)

	switch e.State {
	case libcontainerd.StateExit:
		p, err := pm.config.Store.GetV2Plugin(id)
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

		os.RemoveAll(filepath.Join(pm.config.ExecRoot, id))

		if p.PropagatedMount != "" {
			if err := mount.Unmount(p.PropagatedMount); err != nil {
				logrus.Warnf("Could not unmount %s: %v", p.PropagatedMount, err)
			}
			propRoot := filepath.Join(filepath.Dir(p.Rootfs), "propagated-mount")
			if err := mount.Unmount(propRoot); err != nil {
				logrus.Warn("Could not unmount %s: %v", propRoot, err)
			}
		}

		if restart {
			pm.enable(p, c, true)
		}
	}

	return nil
}

func (pm *Manager) reload() error { // todo: restore
	dir, err := ioutil.ReadDir(pm.config.Root)
	if err != nil {
		return errors.Wrapf(err, "failed to read %v", pm.config.Root)
	}
	plugins := make(map[string]*v2.Plugin)
	for _, v := range dir {
		if validFullID.MatchString(v.Name()) {
			p, err := pm.loadPlugin(v.Name())
			if err != nil {
				return err
			}
			plugins[p.GetID()] = p
		}
	}

	pm.config.Store.SetAll(plugins)

	var wg sync.WaitGroup
	wg.Add(len(plugins))
	for _, p := range plugins {
		c := &controller{} // todo: remove this
		pm.cMap[p] = c
		go func(p *v2.Plugin) {
			defer wg.Done()
			if err := pm.restorePlugin(p); err != nil {
				logrus.Errorf("failed to restore plugin '%s': %s", p.Name(), err)
				return
			}

			if p.Rootfs != "" {
				p.Rootfs = filepath.Join(pm.config.Root, p.PluginObj.ID, "rootfs")
			}

			// We should only enable rootfs propagation for certain plugin types that need it.
			for _, typ := range p.PluginObj.Config.Interface.Types {
				if (typ.Capability == "volumedriver" || typ.Capability == "graphdriver") && typ.Prefix == "docker" && strings.HasPrefix(typ.Version, "1.") {
					if p.PluginObj.Config.PropagatedMount != "" {
						propRoot := filepath.Join(filepath.Dir(p.Rootfs), "propagated-mount")

						// check if we need to migrate an older propagated mount from before
						// these mounts were stored outside the plugin rootfs
						if _, err := os.Stat(propRoot); os.IsNotExist(err) {
							if _, err := os.Stat(p.PropagatedMount); err == nil {
								// make sure nothing is mounted here
								// don't care about errors
								mount.Unmount(p.PropagatedMount)
								if err := os.Rename(p.PropagatedMount, propRoot); err != nil {
									logrus.WithError(err).WithField("dir", propRoot).Error("error migrating propagated mount storage")
								}
								if err := os.MkdirAll(p.PropagatedMount, 0755); err != nil {
									logrus.WithError(err).WithField("dir", p.PropagatedMount).Error("error migrating propagated mount storage")
								}
							}
						}

						if err := os.MkdirAll(propRoot, 0755); err != nil {
							logrus.Errorf("failed to create PropagatedMount directory at %s: %v", propRoot, err)
						}
						// TODO: sanitize PropagatedMount and prevent breakout
						p.PropagatedMount = filepath.Join(p.Rootfs, p.PluginObj.Config.PropagatedMount)
						if err := os.MkdirAll(p.PropagatedMount, 0755); err != nil {
							logrus.Errorf("failed to create PropagatedMount directory at %s: %v", p.PropagatedMount, err)
							return
						}
					}
				}
			}

			pm.save(p)
			requiresManualRestore := !pm.config.LiveRestoreEnabled && p.IsEnabled()

			if requiresManualRestore {
				// if liveRestore is not enabled, the plugin will be stopped now so we should enable it
				if err := pm.enable(p, c, true); err != nil {
					logrus.Errorf("failed to enable plugin '%s': %s", p.Name(), err)
				}
			}
		}(p)
	}
	wg.Wait()
	return nil
}

func (pm *Manager) loadPlugin(id string) (*v2.Plugin, error) {
	p := filepath.Join(pm.config.Root, id, configFileName)
	dt, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading %v", p)
	}
	var plugin v2.Plugin
	if err := json.Unmarshal(dt, &plugin); err != nil {
		return nil, errors.Wrapf(err, "error decoding %v", p)
	}
	return &plugin, nil
}

func (pm *Manager) save(p *v2.Plugin) error {
	pluginJSON, err := json.Marshal(p)
	if err != nil {
		return errors.Wrap(err, "failed to marshal plugin json")
	}
	if err := ioutils.AtomicWriteFile(filepath.Join(pm.config.Root, p.GetID(), configFileName), pluginJSON, 0600); err != nil {
		return errors.Wrap(err, "failed to write atomically plugin json")
	}
	return nil
}

// GC cleans up unrefrenced blobs. This is recommended to run in a goroutine
func (pm *Manager) GC() {
	pm.muGC.Lock()
	defer pm.muGC.Unlock()

	whitelist := make(map[digest.Digest]struct{})
	for _, p := range pm.config.Store.GetAll() {
		whitelist[p.Config] = struct{}{}
		for _, b := range p.Blobsums {
			whitelist[b] = struct{}{}
		}
	}

	pm.blobStore.gc(whitelist)
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

func validatePrivileges(requiredPrivileges, privileges types.PluginPrivileges) error {
	// todo: make a better function that doesn't check order
	if !reflect.DeepEqual(privileges, requiredPrivileges) {
		return errors.New("incorrect privileges")
	}
	return nil
}

func configToRootFS(c []byte) (*image.RootFS, error) {
	var pluginConfig types.PluginConfig
	if err := json.Unmarshal(c, &pluginConfig); err != nil {
		return nil, err
	}
	// validation for empty rootfs is in distribution code
	if pluginConfig.Rootfs == nil {
		return nil, nil
	}

	return rootFSFromPlugin(pluginConfig.Rootfs), nil
}

func rootFSFromPlugin(pluginfs *types.PluginConfigRootfs) *image.RootFS {
	rootFS := image.RootFS{
		Type:    pluginfs.Type,
		DiffIDs: make([]layer.DiffID, len(pluginfs.DiffIds)),
	}
	for i := range pluginfs.DiffIds {
		rootFS.DiffIDs[i] = layer.DiffID(pluginfs.DiffIds[i])
	}

	return &rootFS
}
