// +build windows,experimental

package plugin

import (
	"fmt"

	"github.com/opencontainers/specs/specs-go"
)

func (pm *Manager) enable(p *plugin, force bool) error {
	return fmt.Errorf("Not implemented")
}

func (pm *Manager) initSpec(p *plugin) (*specs.Spec, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (pm *Manager) disable(p *plugin) error {
	return fmt.Errorf("Not implemented")
}

func (pm *Manager) restore(p *plugin) error {
	return fmt.Errorf("Not implemented")
}

// Shutdown plugins
func (pm *Manager) Shutdown() {
}
