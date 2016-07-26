// +build experimental

package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/restartmanager"
	"github.com/docker/engine-api/types"
)

const defaultPluginRuntimeDestination = "/run/docker/plugins"

var manager *Manager

// ErrNotFound indicates that a plugin was not found locally.
type ErrNotFound string

func (name ErrNotFound) Error() string { return fmt.Sprintf("plugin %q not found", string(name)) }

// ErrInadequateCapability indicates that a plugin was found but did not have the requested capability.
type ErrInadequateCapability struct {
	name       string
	capability string
}

func (e ErrInadequateCapability) Error() string {
	return fmt.Sprintf("plugin %q found, but not with %q capability", e.name, e.capability)
}

type plugin struct {
	//sync.RWMutex TODO
	P                 types.Plugin `json:"plugin"`
	client            *plugins.Client
	restartManager    restartmanager.RestartManager
	runtimeSourcePath string
	exitChan          chan bool
}

func (p *plugin) Client() *plugins.Client {
	return p.client
}

// IsLegacy returns true for legacy plugins and false otherwise.
func (p *plugin) IsLegacy() bool {
	return false
}

func (p *plugin) Name() string {
	name := p.P.Name
	if len(p.P.Tag) > 0 {
		// TODO: this feels hacky, maybe we should be storing the distribution reference rather than splitting these
		name += ":" + p.P.Tag
	}
	return name
}

func (pm *Manager) newPlugin(ref reference.Named, id string) *plugin {
	p := &plugin{
		P: types.Plugin{
			Name: ref.Name(),
			ID:   id,
		},
		runtimeSourcePath: filepath.Join(pm.runRoot, id),
	}
	if ref, ok := ref.(reference.NamedTagged); ok {
		p.P.Tag = ref.Tag()
	}
	return p
}

func (pm *Manager) restorePlugin(p *plugin) error {
	p.runtimeSourcePath = filepath.Join(pm.runRoot, p.P.ID)
	if p.P.Active {
		return pm.restore(p)
	}
	return nil
}

type pluginMap map[string]*plugin

// Manager controls the plugin subsystem.
type Manager struct {
	sync.RWMutex
	libRoot          string
	runRoot          string
	plugins          pluginMap // TODO: figure out why save() doesn't json encode *plugin object
	nameToID         map[string]string
	handlers         map[string]func(string, *plugins.Client)
	containerdClient libcontainerd.Client
	registryService  registry.Service
	handleLegacy     bool
	liveRestore      bool
	shutdown         bool
}

// GetManager returns the singleton plugin Manager
func GetManager() *Manager {
	return manager
}

// Init (was NewManager) instantiates the singleton Manager.
// TODO: revert this to NewManager once we get rid of all the singletons.
func Init(root string, remote libcontainerd.Remote, rs registry.Service, liveRestore bool) (err error) {
	if manager != nil {
		return nil
	}

	root = filepath.Join(root, "plugins")
	manager = &Manager{
		libRoot:         root,
		runRoot:         "/run/docker",
		plugins:         make(map[string]*plugin),
		nameToID:        make(map[string]string),
		handlers:        make(map[string]func(string, *plugins.Client)),
		registryService: rs,
		handleLegacy:    true,
		liveRestore:     liveRestore,
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

// Handle sets a callback for a given capability. The callback will be called for every plugin with a given capability.
// TODO: append instead of set?
func Handle(capability string, callback func(string, *plugins.Client)) {
	pluginType := fmt.Sprintf("docker.%s/1", strings.ToLower(capability))
	manager.handlers[pluginType] = callback
	if manager.handleLegacy {
		plugins.Handle(capability, callback)
	}
}

func (pm *Manager) get(name string) (*plugin, error) {
	pm.RLock()
	defer pm.RUnlock()

	id, nameOk := pm.nameToID[name]
	if !nameOk {
		return nil, ErrNotFound(name)
	}

	p, idOk := pm.plugins[id]
	if !idOk {
		return nil, ErrNotFound(name)
	}

	return p, nil
}

// FindWithCapability returns a list of plugins matching the given capability.
func FindWithCapability(capability string) ([]Plugin, error) {
	handleLegacy := true
	result := make([]Plugin, 0, 1)
	if manager != nil {
		handleLegacy = manager.handleLegacy
		manager.RLock()
		defer manager.RUnlock()
	pluginLoop:
		for _, p := range manager.plugins {
			for _, typ := range p.P.Manifest.Interface.Types {
				if typ.Capability != capability || typ.Prefix != "docker" {
					continue pluginLoop
				}
			}
			result = append(result, p)
		}
	}
	if handleLegacy {
		pl, err := plugins.GetAll(capability)
		if err != nil {
			return nil, fmt.Errorf("legacy plugin: %v", err)
		}
		for _, p := range pl {
			if _, ok := manager.nameToID[p.Name()]; !ok {
				result = append(result, p)
			}
		}
	}
	return result, nil
}

// LookupWithCapability returns a plugin matching the given name and capability.
func LookupWithCapability(name, capability string) (Plugin, error) {
	var (
		p   *plugin
		err error
	)
	handleLegacy := true
	if manager != nil {
		fullName := name
		if named, err := reference.ParseNamed(fullName); err == nil { // FIXME: validate
			if reference.IsNameOnly(named) {
				named = reference.WithDefaultTag(named)
			}
			ref, ok := named.(reference.NamedTagged)
			if !ok {
				return nil, fmt.Errorf("invalid name: %s", named.String())
			}
			fullName = ref.String()
		}
		p, err = manager.get(fullName)
		if err != nil {
			if _, ok := err.(ErrNotFound); !ok {
				return nil, err
			}
			handleLegacy = manager.handleLegacy
		} else {
			handleLegacy = false
		}
	}
	if handleLegacy {
		p, err := plugins.Get(name, capability)
		if err != nil {
			return nil, fmt.Errorf("legacy plugin: %v", err)
		}
		return p, nil
	} else if err != nil {
		return nil, err
	}

	capability = strings.ToLower(capability)
	for _, typ := range p.P.Manifest.Interface.Types {
		if typ.Capability == capability && typ.Prefix == "docker" {
			return p, nil
		}
	}
	return nil, ErrInadequateCapability{name, capability}
}

// StateChanged updates plugin internals using from libcontainerd events.
func (pm *Manager) StateChanged(id string, e libcontainerd.StateInfo) error {
	logrus.Debugf("plugin state changed %s %#v", id, e)

	switch e.State {
	case libcontainerd.StateExit:
		pm.RLock()
		p, idOk := pm.plugins[id]
		pm.RUnlock()
		if !idOk {
			return ErrNotFound(id)
		}
		if pm.shutdown == true {
			p.exitChan <- true
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

	if err := json.NewDecoder(dt).Decode(&pm.plugins); err != nil {
		return err
	}

	var group sync.WaitGroup
	group.Add(len(pm.plugins))
	for _, p := range pm.plugins {
		go func(p *plugin) {
			defer group.Done()
			if err := pm.restorePlugin(p); err != nil {
				logrus.Errorf("Error restoring plugin '%s': %s", p.Name(), err)
				return
			}

			pm.Lock()
			pm.nameToID[p.Name()] = p.P.ID
			requiresManualRestore := !pm.liveRestore && p.P.Active
			pm.Unlock()

			if requiresManualRestore {
				// if liveRestore is not enabled, the plugin will be stopped now so we should enable it
				if err := pm.enable(p); err != nil {
					logrus.Errorf("Error enabling plugin '%s': %s", p.Name(), err)
				}
			}
		}(p)
		group.Wait()
	}
	return pm.save()
}

func (pm *Manager) initPlugin(p *plugin) error {
	dt, err := os.Open(filepath.Join(pm.libRoot, p.P.ID, "manifest.json"))
	if err != nil {
		return err
	}
	err = json.NewDecoder(dt).Decode(&p.P.Manifest)
	dt.Close()
	if err != nil {
		return err
	}

	p.P.Config.Mounts = make([]types.PluginMount, len(p.P.Manifest.Mounts))
	for i, mount := range p.P.Manifest.Mounts {
		p.P.Config.Mounts[i] = mount
	}
	p.P.Config.Env = make([]string, 0, len(p.P.Manifest.Env))
	for _, env := range p.P.Manifest.Env {
		if env.Value != nil {
			p.P.Config.Env = append(p.P.Config.Env, fmt.Sprintf("%s=%s", env.Name, *env.Value))
		}
	}
	copy(p.P.Config.Args, p.P.Manifest.Args.Value)

	f, err := os.Create(filepath.Join(pm.libRoot, p.P.ID, "plugin-config.json"))
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(&p.P.Config)
	f.Close()
	return err
}

func (pm *Manager) remove(p *plugin) error {
	if p.P.Active {
		return fmt.Errorf("plugin %s is active", p.Name())
	}
	pm.Lock() // fixme: lock single record
	defer pm.Unlock()
	delete(pm.plugins, p.P.ID)
	delete(pm.nameToID, p.Name())
	pm.save()
	return nil
}

func (pm *Manager) set(p *plugin, args []string) error {
	m := make(map[string]string, len(args))
	for _, arg := range args {
		i := strings.Index(arg, "=")
		if i < 0 {
			return fmt.Errorf("No equal sign '=' found in %s", arg)
		}
		m[arg[:i]] = arg[i+1:]
	}
	return errors.New("not implemented")
}

// fixme: not safe
func (pm *Manager) save() error {
	filePath := filepath.Join(pm.libRoot, "plugins.json")

	jsonData, err := json.Marshal(pm.plugins)
	if err != nil {
		logrus.Debugf("Error in json.Marshal: %v", err)
		return err
	}
	ioutils.AtomicWriteFile(filePath, jsonData, 0600)
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

func computePrivileges(m *types.PluginManifest) types.PluginPrivileges {
	var privileges types.PluginPrivileges
	if m.Network.Type != "null" && m.Network.Type != "bridge" {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "network",
			Description: "",
			Value:       []string{m.Network.Type},
		})
	}
	for _, mount := range m.Mounts {
		if mount.Source != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "mount",
				Description: "",
				Value:       []string{*mount.Source},
			})
		}
	}
	for _, device := range m.Devices {
		if device.Path != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "device",
				Description: "",
				Value:       []string{*device.Path},
			})
		}
	}
	if len(m.Capabilities) > 0 {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "capabilities",
			Description: "",
			Value:       m.Capabilities,
		})
	}
	return privileges
}
