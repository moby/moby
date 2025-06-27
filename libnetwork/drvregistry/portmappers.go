package drvregistry

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/libnetwork/portmapperapi"
)

var _ portmapperapi.Registerer = (*PortMappers)(nil)

type PortMappers struct {
	drivers map[string]portmapperapi.PortMapper
}

// Register a portmapper driver with the registry.
func (r *PortMappers) Register(name string, pm portmapperapi.PortMapper) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("portmapper name cannot be empty")
	}

	if _, ok := r.drivers[name]; ok {
		return errors.New("portmapper already registered")
	}

	if r.drivers == nil {
		r.drivers = make(map[string]portmapperapi.PortMapper)
	}

	r.drivers[name] = pm

	return nil
}

// Get retrieves a portmapper driver by name from the registry.
func (r *PortMappers) Get(name string) (portmapperapi.PortMapper, error) {
	pm, ok := r.drivers[name]
	if !ok {
		return nil, fmt.Errorf("portmapper %s not found", name)
	}
	return pm, nil
}
