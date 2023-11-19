/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package plugin

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrNoType is returned when no type is specified
	ErrNoType = errors.New("plugin: no type")
	// ErrNoPluginID is returned when no id is specified
	ErrNoPluginID = errors.New("plugin: no id")
	// ErrIDRegistered is returned when a duplicate id is already registered
	ErrIDRegistered = errors.New("plugin: id already registered")
	// ErrSkipPlugin is used when a plugin is not initialized and should not be loaded,
	// this allows the plugin loader differentiate between a plugin which is configured
	// not to load and one that fails to load.
	ErrSkipPlugin = errors.New("skip plugin")

	// ErrInvalidRequires will be thrown if the requirements for a plugin are
	// defined in an invalid manner.
	ErrInvalidRequires = errors.New("invalid requires")
)

// IsSkipPlugin returns true if the error is skipping the plugin
func IsSkipPlugin(err error) bool {
	return errors.Is(err, ErrSkipPlugin)
}

// Type is the type of the plugin
type Type string

func (t Type) String() string { return string(t) }

const (
	// InternalPlugin implements an internal plugin to containerd
	InternalPlugin Type = "io.containerd.internal.v1"
	// RuntimePlugin implements a runtime
	RuntimePlugin Type = "io.containerd.runtime.v1"
	// RuntimePluginV2 implements a runtime v2
	RuntimePluginV2 Type = "io.containerd.runtime.v2"
	// ServicePlugin implements a internal service
	ServicePlugin Type = "io.containerd.service.v1"
	// GRPCPlugin implements a grpc service
	GRPCPlugin Type = "io.containerd.grpc.v1"
	// TTRPCPlugin implements a ttrpc shim service
	TTRPCPlugin Type = "io.containerd.ttrpc.v1"
	// SnapshotPlugin implements a snapshotter
	SnapshotPlugin Type = "io.containerd.snapshotter.v1"
	// TaskMonitorPlugin implements a task monitor
	TaskMonitorPlugin Type = "io.containerd.monitor.v1"
	// DiffPlugin implements a differ
	DiffPlugin Type = "io.containerd.differ.v1"
	// MetadataPlugin implements a metadata store
	MetadataPlugin Type = "io.containerd.metadata.v1"
	// ContentPlugin implements a content store
	ContentPlugin Type = "io.containerd.content.v1"
	// GCPlugin implements garbage collection policy
	GCPlugin Type = "io.containerd.gc.v1"
	// EventPlugin implements event handling
	EventPlugin Type = "io.containerd.event.v1"
	// LeasePlugin implements lease manager
	LeasePlugin Type = "io.containerd.lease.v1"
	// Streaming implements a stream manager
	StreamingPlugin Type = "io.containerd.streaming.v1"
	// TracingProcessorPlugin implements a open telemetry span processor
	TracingProcessorPlugin Type = "io.containerd.tracing.processor.v1"
	// NRIApiPlugin implements the NRI adaptation interface for containerd.
	NRIApiPlugin Type = "io.containerd.nri.v1"
	// TransferPlugin implements a transfer service
	TransferPlugin Type = "io.containerd.transfer.v1"
	// SandboxStorePlugin implements a sandbox store
	SandboxStorePlugin Type = "io.containerd.sandbox.store.v1"
	// SandboxControllerPlugin implements a sandbox controller
	SandboxControllerPlugin Type = "io.containerd.sandbox.controller.v1"
	// WarningPlugin implements a warning service
	WarningPlugin Type = "io.containerd.warning.v1"
)

const (
	// RuntimeLinuxV1 is the legacy linux runtime
	RuntimeLinuxV1 = "io.containerd.runtime.v1.linux"
	// RuntimeRuncV1 is the runc runtime that supports a single container
	RuntimeRuncV1 = "io.containerd.runc.v1"
	// RuntimeRuncV2 is the runc runtime that supports multiple containers per shim
	RuntimeRuncV2      = "io.containerd.runc.v2"
	DeprecationsPlugin = "deprecations"
)

// Registration contains information for registering a plugin
type Registration struct {
	// Type of the plugin
	Type Type
	// ID of the plugin
	ID string
	// Config specific to the plugin
	Config interface{}
	// Requires is a list of plugins that the registered plugin requires to be available
	Requires []Type

	// InitFn is called when initializing a plugin. The registration and
	// context are passed in. The init function may modify the registration to
	// add exports, capabilities and platform support declarations.
	InitFn func(*InitContext) (interface{}, error)
	// Disable the plugin from loading
	Disable bool
}

// Init the registered plugin
func (r *Registration) Init(ic *InitContext) *Plugin {
	p, err := r.InitFn(ic)
	return &Plugin{
		Registration: r,
		Config:       ic.Config,
		Meta:         ic.Meta,
		instance:     p,
		err:          err,
	}
}

// URI returns the full plugin URI
func (r *Registration) URI() string {
	return fmt.Sprintf("%s.%s", r.Type, r.ID)
}

var register = struct {
	sync.RWMutex
	r []*Registration
}{}

// Load loads all plugins at the provided path into containerd
func Load(path string) (count int, err error) {
	defer func() {
		if v := recover(); v != nil {
			rerr, ok := v.(error)
			if !ok {
				rerr = fmt.Errorf("%s", v)
			}
			err = rerr
		}
	}()
	return loadPlugins(path)
}

// Register allows plugins to register
func Register(r *Registration) {
	register.Lock()
	defer register.Unlock()

	if r.Type == "" {
		panic(ErrNoType)
	}
	if r.ID == "" {
		panic(ErrNoPluginID)
	}
	if err := checkUnique(r); err != nil {
		panic(err)
	}

	for _, requires := range r.Requires {
		if requires == "*" && len(r.Requires) != 1 {
			panic(ErrInvalidRequires)
		}
	}

	register.r = append(register.r, r)
}

func checkUnique(r *Registration) error {
	for _, registered := range register.r {
		if r.URI() == registered.URI() {
			return fmt.Errorf("%s: %w", r.URI(), ErrIDRegistered)
		}
	}
	return nil
}

// DisableFilter filters out disabled plugins
type DisableFilter func(r *Registration) bool

// Graph returns an ordered list of registered plugins for initialization.
// Plugins in disableList specified by id will be disabled.
func Graph(filter DisableFilter) (ordered []*Registration) {
	register.RLock()
	defer register.RUnlock()

	for _, r := range register.r {
		if filter(r) {
			r.Disable = true
		}
	}

	added := map[*Registration]bool{}
	for _, r := range register.r {
		if r.Disable {
			continue
		}
		children(r, added, &ordered)
		if !added[r] {
			ordered = append(ordered, r)
			added[r] = true
		}
	}
	return ordered
}

func children(reg *Registration, added map[*Registration]bool, ordered *[]*Registration) {
	for _, t := range reg.Requires {
		for _, r := range register.r {
			if !r.Disable &&
				r.URI() != reg.URI() &&
				(t == "*" || r.Type == t) {
				children(r, added, ordered)
				if !added[r] {
					*ordered = append(*ordered, r)
					added[r] = true
				}
			}
		}
	}
}
