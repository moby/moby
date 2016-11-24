package v2

import (
	"encoding/json"
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
	sync.RWMutex
	PluginObj         types.Plugin    `json:"plugin"`
	PClient           *plugins.Client `json:"-"`
	RuntimeSourcePath string          `json:"-"`
	RefCount          int             `json:"-"`
	Restart           bool            `json:"-"`
	ExitChan          chan bool       `json:"-"`
	LibRoot           string          `json:"-"`
	TimeoutInSecs     int             `json:"-"`
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
		RuntimeSourcePath: filepath.Join(runRoot, id),
		LibRoot:           libRoot,
	}
}

// Client returns the plugin client.
func (p *Plugin) Client() *plugins.Client {
	return p.PClient
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
	return os.RemoveAll(p.RuntimeSourcePath)
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
	for i, mount := range p.PluginObj.Config.Mounts {
		p.PluginObj.Settings.Mounts[i] = mount
	}
	p.PluginObj.Settings.Env = make([]string, 0, len(p.PluginObj.Config.Env))
	p.PluginObj.Settings.Devices = make([]types.PluginDevice, 0, len(p.PluginObj.Config.Linux.Devices))
	copy(p.PluginObj.Settings.Devices, p.PluginObj.Config.Linux.Devices)
	for _, env := range p.PluginObj.Config.Env {
		if env.Value != nil {
			p.PluginObj.Settings.Env = append(p.PluginObj.Settings.Env, fmt.Sprintf("%s=%s", env.Name, *env.Value))
		}
	}
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
	p.Lock()
	defer p.Unlock()

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
	p.RLock()
	defer p.RUnlock()

	return p.PluginObj.Enabled
}

// GetID returns the plugin's ID.
func (p *Plugin) GetID() string {
	p.RLock()
	defer p.RUnlock()

	return p.PluginObj.ID
}

// GetSocket returns the plugin socket.
func (p *Plugin) GetSocket() string {
	p.RLock()
	defer p.RUnlock()

	return p.PluginObj.Config.Interface.Socket
}

// GetTypes returns the interface types of a plugin.
func (p *Plugin) GetTypes() []types.PluginInterfaceType {
	p.RLock()
	defer p.RUnlock()

	return p.PluginObj.Config.Interface.Types
}

// InitSpec creates an OCI spec from the plugin's config.
func (p *Plugin) InitSpec(s specs.Spec, libRoot string) (*specs.Spec, error) {
	rootfs := filepath.Join(libRoot, p.PluginObj.ID, "rootfs")
	s.Root = specs.Root{
		Path:     rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in config?
	}

	userMounts := make(map[string]struct{}, len(p.PluginObj.Config.Mounts))
	for _, m := range p.PluginObj.Config.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	mounts := append(p.PluginObj.Config.Mounts, types.PluginMount{
		Source:      &p.RuntimeSourcePath,
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

	for _, mount := range mounts {
		m := specs.Mount{
			Destination: mount.Destination,
			Type:        mount.Type,
			Options:     mount.Options,
		}
		// TODO: if nil, then it's required and user didn't set it
		if mount.Source != nil {
			m.Source = *mount.Source
		}
		if m.Source != "" && m.Type == "bind" {
			fi, err := os.Lstat(filepath.Join(rootfs, m.Destination)) // TODO: followsymlinks
			if err != nil {
				return nil, err
			}
			if fi.IsDir() {
				if err := os.MkdirAll(m.Source, 0700); err != nil {
					return nil, err
				}
			}
		}
		s.Mounts = append(s.Mounts, m)
	}

	for i, m := range s.Mounts {
		if strings.HasPrefix(m.Destination, "/dev/") {
			if _, ok := userMounts[m.Destination]; ok {
				s.Mounts = append(s.Mounts[:i], s.Mounts[i+1:]...)
			}
		}
	}

	if p.PluginObj.Config.Linux.DeviceCreation {
		rwm := "rwm"
		s.Linux.Resources.Devices = []specs.DeviceCgroup{{Allow: true, Access: &rwm}}
	}
	for _, dev := range p.PluginObj.Config.Linux.Devices {
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
