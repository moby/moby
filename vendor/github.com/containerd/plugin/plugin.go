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
	"context"
	"errors"
	"fmt"
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
	// ErrPluginInitialized is used when a plugin is already initialized
	ErrPluginInitialized = errors.New("plugin: already initialized")
	// ErrPluginNotFound is used when a plugin is looked up but not found
	ErrPluginNotFound = errors.New("plugin: not found")
	// ErrPluginMultipleInstances is used when a plugin is expected a single instance but has multiple
	ErrPluginMultipleInstances = errors.New("plugin: multiple instances")
	// ErrPluginCircularDependency is used when the graph detect a circular plugin dependency
	ErrPluginCircularDependency = errors.New("plugin: dependency loop detected")

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

	// ConfigMigration allows a plugin to migrate configurations from an older
	// version to handle plugin renames or moving of features from one plugin
	// to another in a later version.
	// The configuration map is keyed off the plugin name and the value
	// is the configuration for that objects, with the structure defined
	// for the plugin. No validation is done on the value before performing
	// the migration.
	ConfigMigration func(context.Context, int, map[string]interface{}) error
}

// Init the registered plugin
func (r Registration) Init(ic *InitContext) *Plugin {
	p, err := r.InitFn(ic)
	return &Plugin{
		Registration: r,
		Config:       ic.Config,
		Meta:         *ic.Meta,
		instance:     p,
		err:          err,
	}
}

// URI returns the full plugin URI
func (r *Registration) URI() string {
	return r.Type.String() + "." + r.ID
}

// DisableFilter filters out disabled plugins
type DisableFilter func(r *Registration) bool

// Registry is list of registrations which can be registered to and
// produce a filtered and ordered output.
// The Registry itself is immutable and the list will be copied
// and appeneded to a new registry when new items are registered.
type Registry []*Registration

// Graph computes the ordered list of registrations based on their dependencies,
// filtering out any plugins which match the provided filter.
func (registry Registry) Graph(filter DisableFilter) []Registration {
	handled := make(map[*Registration]struct{}, len(registry))
	if filter != nil {
		for _, r := range registry {
			if filter(r) {
				handled[r] = struct{}{}
			}
		}
	}

	ordered := make([]Registration, 0, len(registry)-len(handled))
	stack := make([]*Registration, 0, cap(ordered))
	for _, r := range registry {
		if _, ok := handled[r]; ok {
			continue
		}
		children(append(stack, r), registry, handled, &ordered)
		handled[r] = struct{}{}
		ordered = append(ordered, *r)
	}
	return ordered
}

func children(stack []*Registration, registry []*Registration, handled map[*Registration]struct{}, ordered *[]Registration) {
	reg := stack[len(stack)-1]
	for _, t := range reg.Requires {
		for _, r := range registry {
			if (t == "*" || r.Type == t) && r != reg {
				if _, ok := handled[r]; !ok {
					// Ensure not in current stack
					for _, p := range stack[:len(stack)-1] {
						if p == r {
							panic(fmt.Errorf("circular plugin dependency at %s: %w", r.URI(), ErrPluginCircularDependency))
						}
					}
					children(append(stack, r), registry, handled, ordered)
					handled[r] = struct{}{}
					*ordered = append(*ordered, *r)
				}
			}
		}
	}
}

// Register adds the registration to a Registry and returns the
// updated Registry, panicking if registration could not succeed.
func (registry Registry) Register(r *Registration) Registry {
	if r.Type == "" {
		panic(ErrNoType)
	}
	if r.ID == "" {
		panic(ErrNoPluginID)
	}
	if err := checkUnique(registry, r); err != nil {
		panic(err)
	}

	for _, requires := range r.Requires {
		if (requires == "*" && len(r.Requires) != 1) || requires == r.Type {
			panic(ErrInvalidRequires)
		}
	}

	return append(registry, r)
}

func checkUnique(registry Registry, r *Registration) error {
	for _, registered := range registry {
		if r.Type == registered.Type && r.ID == registered.ID {
			return fmt.Errorf("%s: %w", r.URI(), ErrIDRegistered)
		}
	}
	return nil
}
