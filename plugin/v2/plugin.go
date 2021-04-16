package v2 // import "github.com/docker/docker/plugin/v2"

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Plugin represents an individual plugin.
type Plugin struct {
	mu        sync.RWMutex
	PluginObj types.Plugin `json:"plugin"` // todo: embed struct
	pClient   *plugins.Client
	refCount  int
	Rootfs    string // TODO: make private

	Config   digest.Digest
	Blobsums []digest.Digest
	Manifest digest.Digest

	modifyRuntimeSpec func(*specs.Spec)

	SwarmServiceID string
	timeout        time.Duration
	addr           net.Addr
}

const defaultPluginRuntimeDestination = "/run/docker/plugins"

// ErrInadequateCapability indicates that the plugin did not have the requested capability.
type ErrInadequateCapability struct {
	cap string
}

func (e ErrInadequateCapability) Error() string {
	return fmt.Sprintf("plugin does not provide %q capability", e.cap)
}

// ScopedPath returns the path scoped to the plugin rootfs
func (p *Plugin) ScopedPath(s string) string {
	if p.PluginObj.Config.PropagatedMount != "" && strings.HasPrefix(s, p.PluginObj.Config.PropagatedMount) {
		// re-scope to the propagated mount path on the host
		return filepath.Join(filepath.Dir(p.Rootfs), "propagated-mount", strings.TrimPrefix(s, p.PluginObj.Config.PropagatedMount))
	}
	return filepath.Join(p.Rootfs, s)
}

// Client returns the plugin client.
// Deprecated: use p.Addr() and manually create the client
func (p *Plugin) Client() *plugins.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.pClient
}

// SetPClient set the plugin client.
// Deprecated: Hardcoded plugin client is deprecated
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
	return p.PluginObj.Name
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

// InitEmptySettings initializes empty settings for a plugin.
func (p *Plugin) InitEmptySettings() {
	p.PluginObj.Settings.Mounts = make([]types.PluginMount, len(p.PluginObj.Config.Mounts))
	copy(p.PluginObj.Settings.Mounts, p.PluginObj.Config.Mounts)
	p.PluginObj.Settings.Devices = make([]types.PluginDevice, len(p.PluginObj.Config.Linux.Devices))
	copy(p.PluginObj.Settings.Devices, p.PluginObj.Config.Linux.Devices)
	p.PluginObj.Settings.Env = make([]string, 0, len(p.PluginObj.Config.Env))
	for _, env := range p.PluginObj.Config.Env {
		if env.Value != nil {
			p.PluginObj.Settings.Env = append(p.PluginObj.Settings.Env, fmt.Sprintf("%s=%s", env.Name, *env.Value))
		}
	}
	p.PluginObj.Settings.Args = make([]string, len(p.PluginObj.Config.Args.Value))
	copy(p.PluginObj.Settings.Args, p.PluginObj.Config.Args.Value)
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
	for _, set := range sets {
		s := set

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
				if mount.Source == nil {
					return fmt.Errorf("Plugin config has no mount source")
				}
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
				if device.Path == nil {
					return fmt.Errorf("Plugin config has no device path")
				}
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

	return nil
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

// Acquire increments the plugin's reference count
// This should be followed up by `Release()` when the plugin is no longer in use.
func (p *Plugin) Acquire() {
	p.AddRefCount(plugingetter.Acquire)
}

// Release decrements the plugin's reference count
// This should only be called when the plugin is no longer in use, e.g. with
// via `Acquire()` or getter.Get("name", "type", plugingetter.Acquire)
func (p *Plugin) Release() {
	p.AddRefCount(plugingetter.Release)
}

// SetSpecOptModifier sets the function to use to modify the generated
// runtime spec.
func (p *Plugin) SetSpecOptModifier(f func(*specs.Spec)) {
	p.mu.Lock()
	p.modifyRuntimeSpec = f
	p.mu.Unlock()
}

// Timeout gets the currently configured connection timeout.
// This should be used when dialing the plugin.
func (p *Plugin) Timeout() time.Duration {
	p.mu.RLock()
	t := p.timeout
	p.mu.RUnlock()
	return t
}

// SetTimeout sets the timeout to use for dialing.
func (p *Plugin) SetTimeout(t time.Duration) {
	p.mu.Lock()
	p.timeout = t
	p.mu.Unlock()
}

// Addr returns the net.Addr to use to connect to the plugin socket
func (p *Plugin) Addr() net.Addr {
	p.mu.RLock()
	addr := p.addr
	p.mu.RUnlock()
	return addr
}

// SetAddr sets the plugin address which can be used for dialing the plugin.
func (p *Plugin) SetAddr(addr net.Addr) {
	p.mu.Lock()
	p.addr = addr
	p.mu.Unlock()
}

// Protocol is the protocol that should be used for interacting with the plugin.
func (p *Plugin) Protocol() string {
	if p.PluginObj.Config.Interface.ProtocolScheme != "" {
		return p.PluginObj.Config.Interface.ProtocolScheme
	}
	return plugins.ProtocolSchemeHTTPV1
}
