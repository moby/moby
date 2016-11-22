package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Plugin represents an individual plugin.
type Plugin struct {
	mu                sync.RWMutex
	PluginObj         types.Plugin `json:"plugin"`
	pClient           *plugins.Client
	runtimeSourcePath string
	refCount          int
	LibRoot           string // TODO: make private
	PropagatedMount   string // TODO: make private
	Rootfs            string // TODO: make private
}

const defaultPluginRuntimeDestination = "/run/docker/plugins"

// ErrInadequateCapability indicates that the plugin did not have the requested capability.
type ErrInadequateCapability struct {
	cap string
}

func (e ErrInadequateCapability) Error() string {
	return fmt.Sprintf("plugin does not provide %q capability", e.cap)
}

func newPluginObj(name, id, tag string) types.Plugin {
	return types.Plugin{Name: name, ID: id, Tag: tag}
}

// NewPlugin creates a plugin.
func NewPlugin(name, id, runRoot, libRoot, tag string) *Plugin {
	return &Plugin{
		PluginObj:         newPluginObj(name, id, tag),
		runtimeSourcePath: filepath.Join(runRoot, id),
		LibRoot:           libRoot,
	}
}

// Restore restores the plugin
func (p *Plugin) Restore(runRoot string) {
	p.runtimeSourcePath = filepath.Join(runRoot, p.GetID())
}

// GetRuntimeSourcePath gets the Source (host) path of the plugin socket
// This path gets bind mounted into the plugin.
func (p *Plugin) GetRuntimeSourcePath() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.runtimeSourcePath
}

// BasePath returns the path to which all paths returned by the plugin are relative to.
// For Plugin objects this returns the host path of the plugin container's rootfs.
func (p *Plugin) BasePath() string {
	return p.Rootfs
}

// Client returns the plugin client.
func (p *Plugin) Client() *plugins.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.pClient
}

// SetPClient set the plugin client.
func (p *Plugin) SetPClient(client *plugins.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pClient = client
}

// IsV1 returns true for V1 plugins and false otherwise.
func (p *Plugin) IsV1() bool {
	return false
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	name := p.PluginObj.Name
	if len(p.PluginObj.Tag) > 0 {
		// TODO: this feels hacky, maybe we should be storing the distribution reference rather than splitting these
		name += ":" + p.PluginObj.Tag
	}
	return name
}

// FilterByCap query the plugin for a given capability.
func (p *Plugin) FilterByCap(capability string) (*Plugin, error) {
	capability = strings.ToLower(capability)
	for _, typ := range p.PluginObj.Config.Interface.Types {
		if typ.Capability == capability && typ.Prefix == "docker" {
			return p, nil
		}
	}
	return nil, ErrInadequateCapability{capability}
}

// RemoveFromDisk deletes the plugin's runtime files from disk.
func (p *Plugin) RemoveFromDisk() error {
	return os.RemoveAll(p.runtimeSourcePath)
}

// InitPlugin populates the plugin object from the plugin config file.
func (p *Plugin) InitPlugin() error {
	dt, err := os.Open(filepath.Join(p.LibRoot, p.PluginObj.ID, "config.json"))
	if err != nil {
		return err
	}
	err = json.NewDecoder(dt).Decode(&p.PluginObj.Config)
	dt.Close()
	if err != nil {
		return err
	}

	p.PluginObj.Settings.Mounts = make([]types.PluginMount, len(p.PluginObj.Config.Mounts))
	copy(p.PluginObj.Settings.Mounts, p.PluginObj.Config.Mounts)
	p.PluginObj.Settings.Env = make([]string, 0, len(p.PluginObj.Config.Env))
	p.PluginObj.Settings.Devices = make([]types.PluginDevice, 0, len(p.PluginObj.Config.Linux.Devices))
	copy(p.PluginObj.Settings.Devices, p.PluginObj.Config.Linux.Devices)
	for _, env := range p.PluginObj.Config.Env {
		if env.Value != nil {
			p.PluginObj.Settings.Env = append(p.PluginObj.Settings.Env, fmt.Sprintf("%s=%s", env.Name, *env.Value))
		}
	}
	p.PluginObj.Settings.Args = make([]string, len(p.PluginObj.Config.Args.Value))
	copy(p.PluginObj.Settings.Args, p.PluginObj.Config.Args.Value)

	return p.writeSettings()
}

func (p *Plugin) writeSettings() error {
	f, err := os.Create(filepath.Join(p.LibRoot, p.PluginObj.ID, "plugin-settings.json"))
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(&p.PluginObj.Settings)
	f.Close()
	return err
}

// Set is used to pass arguments to the plugin.
func (p *Plugin) Set(args []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.PluginObj.Enabled {
		return fmt.Errorf("cannot set on an active plugin, disable plugin before setting")
	}

	sets, err := newSettables(args)
	if err != nil {
		return err
	}

	// TODO(vieux): lots of code duplication here, needs to be refactored.

next:
	for _, s := range sets {
		// range over all the envs in the config
		for _, env := range p.PluginObj.Config.Env {
			// found the env in the config
			if env.Name == s.name {
				// is it settable ?
				if ok, err := s.isSettable(allowedSettableFieldsEnv, env.Settable); err != nil {
					return err
				} else if !ok {
					return fmt.Errorf("%q is not settable", s.prettyName())
				}
				// is it, so lets update the settings in memory
				updateSettingsEnv(&p.PluginObj.Settings.Env, &s)
				continue next
			}
		}

		// range over all the mounts in the config
		for _, mount := range p.PluginObj.Config.Mounts {
			// found the mount in the config
			if mount.Name == s.name {
				// is it settable ?
				if ok, err := s.isSettable(allowedSettableFieldsMounts, mount.Settable); err != nil {
					return err
				} else if !ok {
					return fmt.Errorf("%q is not settable", s.prettyName())
				}

				// it is, so lets update the settings in memory
				*mount.Source = s.value
				continue next
			}
		}

		// range over all the devices in the config
		for _, device := range p.PluginObj.Config.Linux.Devices {
			// found the device in the config
			if device.Name == s.name {
				// is it settable ?
				if ok, err := s.isSettable(allowedSettableFieldsDevices, device.Settable); err != nil {
					return err
				} else if !ok {
					return fmt.Errorf("%q is not settable", s.prettyName())
				}

				// it is, so lets update the settings in memory
				*device.Path = s.value
				continue next
			}
		}

		// found the name in the config
		if p.PluginObj.Config.Args.Name == s.name {
			// is it settable ?
			if ok, err := s.isSettable(allowedSettableFieldsArgs, p.PluginObj.Config.Args.Settable); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%q is not settable", s.prettyName())
			}

			// it is, so lets update the settings in memory
			p.PluginObj.Settings.Args = strings.Split(s.value, " ")
			continue next
		}

		return fmt.Errorf("setting %q not found in the plugin configuration", s.name)
	}

	// update the settings on disk
	return p.writeSettings()
}

// IsEnabled returns the active state of the plugin.
func (p *Plugin) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.PluginObj.Enabled
}

// GetID returns the plugin's ID.
func (p *Plugin) GetID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.PluginObj.ID
}

// GetSocket returns the plugin socket.
func (p *Plugin) GetSocket() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.PluginObj.Config.Interface.Socket
}

// GetTypes returns the interface types of a plugin.
func (p *Plugin) GetTypes() []types.PluginInterfaceType {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.PluginObj.Config.Interface.Types
}

// GetRefCount returns the reference count.
func (p *Plugin) GetRefCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.refCount
}

// AddRefCount adds to reference count.
func (p *Plugin) AddRefCount(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.refCount += count
}

// InitSpec creates an OCI spec from the plugin's config.
func (p *Plugin) InitSpec(s specs.Spec) (*specs.Spec, error) {
	s.Root = specs.Root{
		Path:     p.Rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in config?
	}

	userMounts := make(map[string]struct{}, len(p.PluginObj.Settings.Mounts))
	for _, m := range p.PluginObj.Settings.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	if err := os.MkdirAll(p.runtimeSourcePath, 0755); err != nil {
		return nil, err
	}

	mounts := append(p.PluginObj.Config.Mounts, types.PluginMount{
		Source:      &p.runtimeSourcePath,
		Destination: defaultPluginRuntimeDestination,
		Type:        "bind",
		Options:     []string{"rbind", "rshared"},
	})

	if p.PluginObj.Config.Network.Type != "" {
		// TODO: if net == bridge, use libnetwork controller to create a new plugin-specific bridge, bind mount /etc/hosts and /etc/resolv.conf look at the docker code (allocateNetwork, initialize)
		if p.PluginObj.Config.Network.Type == "host" {
			oci.RemoveNamespace(&s, specs.NamespaceType("network"))
		}
		etcHosts := "/etc/hosts"
		resolvConf := "/etc/resolv.conf"
		mounts = append(mounts,
			types.PluginMount{
				Source:      &etcHosts,
				Destination: etcHosts,
				Type:        "bind",
				Options:     []string{"rbind", "ro"},
			},
			types.PluginMount{
				Source:      &resolvConf,
				Destination: resolvConf,
				Type:        "bind",
				Options:     []string{"rbind", "ro"},
			})
	}

	for _, mnt := range mounts {
		m := specs.Mount{
			Destination: mnt.Destination,
			Type:        mnt.Type,
			Options:     mnt.Options,
		}
		if mnt.Source == nil {
			return nil, errors.New("mount source is not specified")
		}
		m.Source = *mnt.Source
		s.Mounts = append(s.Mounts, m)
	}

	for i, m := range s.Mounts {
		if strings.HasPrefix(m.Destination, "/dev/") {
			if _, ok := userMounts[m.Destination]; ok {
				s.Mounts = append(s.Mounts[:i], s.Mounts[i+1:]...)
			}
		}
	}

	if p.PluginObj.Config.PropagatedMount != "" {
		p.PropagatedMount = filepath.Join(p.Rootfs, p.PluginObj.Config.PropagatedMount)
		s.Linux.RootfsPropagation = "rshared"
	}

	if p.PluginObj.Config.Linux.DeviceCreation {
		rwm := "rwm"
		s.Linux.Resources.Devices = []specs.DeviceCgroup{{Allow: true, Access: &rwm}}
	}
	for _, dev := range p.PluginObj.Settings.Devices {
		path := *dev.Path
		d, dPermissions, err := oci.DevicesFromPath(path, path, "rwm")
		if err != nil {
			return nil, err
		}
		s.Linux.Devices = append(s.Linux.Devices, d...)
		s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, dPermissions...)
	}

	envs := make([]string, 1, len(p.PluginObj.Settings.Env)+1)
	envs[0] = "PATH=" + system.DefaultPathEnv
	envs = append(envs, p.PluginObj.Settings.Env...)

	args := append(p.PluginObj.Config.Entrypoint, p.PluginObj.Settings.Args...)
	cwd := p.PluginObj.Config.Workdir
	if len(cwd) == 0 {
		cwd = "/"
	}
	s.Process.Terminal = false
	s.Process.Args = args
	s.Process.Cwd = cwd
	s.Process.Env = envs

	s.Process.Capabilities = append(s.Process.Capabilities, p.PluginObj.Config.Linux.Capabilities...)

	return &s, nil
}
