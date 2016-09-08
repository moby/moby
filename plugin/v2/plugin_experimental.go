// +build experimental

package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const defaultPluginRuntimeDestination = "/run/docker/plugins"

// ErrInadequateCapability indicates that the plugin did not have the requested capability.
type ErrInadequateCapability string

func (cap ErrInadequateCapability) Error() string {
	return fmt.Sprintf("plugin does not provide %q capability", cap)
}

func newPluginObj(name, id, tag string) types.Plugin {
	return types.Plugin{Name: name, ID: id, Tag: tag}
}

// NewPlugin creates a plugin.
func NewPlugin(name, id, runRoot, tag string) *Plugin {
	return &Plugin{
		PluginObj:         newPluginObj(name, id, tag),
		RuntimeSourcePath: filepath.Join(runRoot, id),
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
	for _, typ := range p.PluginObj.Manifest.Interface.Types {
		if typ.Capability == capability && typ.Prefix == "docker" {
			return p, nil
		}
	}
	return nil, ErrInadequateCapability(capability)
}

// RemoveFromDisk deletes the plugin's runtime files from disk.
func (p *Plugin) RemoveFromDisk() error {
	return os.RemoveAll(p.RuntimeSourcePath)
}

// InitPlugin populates the plugin object from the plugin manifest file.
func (p *Plugin) InitPlugin(libRoot string) error {
	dt, err := os.Open(filepath.Join(libRoot, p.PluginObj.ID, "manifest.json"))
	if err != nil {
		return err
	}
	err = json.NewDecoder(dt).Decode(&p.PluginObj.Manifest)
	dt.Close()
	if err != nil {
		return err
	}

	p.PluginObj.Config.Mounts = make([]types.PluginMount, len(p.PluginObj.Manifest.Mounts))
	for i, mount := range p.PluginObj.Manifest.Mounts {
		p.PluginObj.Config.Mounts[i] = mount
	}
	p.PluginObj.Config.Env = make([]string, 0, len(p.PluginObj.Manifest.Env))
	for _, env := range p.PluginObj.Manifest.Env {
		if env.Value != nil {
			p.PluginObj.Config.Env = append(p.PluginObj.Config.Env, fmt.Sprintf("%s=%s", env.Name, *env.Value))
		}
	}
	copy(p.PluginObj.Config.Args, p.PluginObj.Manifest.Args.Value)

	f, err := os.Create(filepath.Join(libRoot, p.PluginObj.ID, "plugin-config.json"))
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(&p.PluginObj.Config)
	f.Close()
	return err
}

// Set is used to pass arguments to the plugin.
func (p *Plugin) Set(args []string) error {
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

// ComputePrivileges takes the manifest file and computes the list of access necessary
// for the plugin on the host.
func (p *Plugin) ComputePrivileges() types.PluginPrivileges {
	m := p.PluginObj.Manifest
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

	return p.PluginObj.Manifest.Interface.Socket
}

// GetTypes returns the interface types of a plugin.
func (p *Plugin) GetTypes() []types.PluginInterfaceType {
	p.RLock()
	defer p.RUnlock()

	return p.PluginObj.Manifest.Interface.Types
}

// InitSpec creates an OCI spec from the plugin's config.
func (p *Plugin) InitSpec(s specs.Spec, libRoot string) (*specs.Spec, error) {
	rootfs := filepath.Join(libRoot, p.PluginObj.ID, "rootfs")
	s.Root = specs.Root{
		Path:     rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in manifest?
	}

	mounts := append(p.PluginObj.Config.Mounts, types.PluginMount{
		Source:      &p.RuntimeSourcePath,
		Destination: defaultPluginRuntimeDestination,
		Type:        "bind",
		Options:     []string{"rbind", "rshared"},
	})
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

	envs := make([]string, 1, len(p.PluginObj.Config.Env)+1)
	envs[0] = "PATH=" + system.DefaultPathEnv
	envs = append(envs, p.PluginObj.Config.Env...)

	args := append(p.PluginObj.Manifest.Entrypoint, p.PluginObj.Config.Args...)
	cwd := p.PluginObj.Manifest.Workdir
	if len(cwd) == 0 {
		cwd = "/"
	}
	s.Process = specs.Process{
		Terminal: false,
		Args:     args,
		Cwd:      cwd,
		Env:      envs,
	}

	return &s, nil
}
