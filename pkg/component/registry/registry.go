package registry

import (
	"fmt"

	"github.com/docker/docker/pkg/component"
)

// Registry stores Components so they can retrieved by other Components
type Registry struct {
	components map[string]component.Component
}

// Register adds a new component to the registry. If there is already a
// component with the same ComponentType an error is returned.
func (r *Registry) Register(c component.Component) error {
	// TODO: validate component

	r.components[c.Provides()] = c
	return nil
}

// Get returns a Component by name that was previously registered
func (r *Registry) Get(name string) (component.Component, error) {
	component, ok := r.components[name]
	if !ok {
		return nil, fmt.Errorf("no component available for %q", name)
	}
	return component, nil
}

// All returns all the Components
func (r *Registry) All() []component.Component {
	comp := []component.Component{}
	for _, component := range r.components {
		comp = append(comp, component)
	}
	return comp
}

// ForEach runs a function on each component.Component
func (r *Registry) ForEach(f func(component.Component) error) error {
	for _, component := range r.components {
		if err := f(component); err != nil {
			return err
		}
	}
	return nil
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{components: make(map[string]component.Component)}
}

var (
	reg *Registry
)

// Get returns the globally registered registry of Components
func Get() *Registry {
	if reg == nil {
		reg = NewRegistry()
	}
	return reg
}
