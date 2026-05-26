//go:build !linux && !windows

package plugin

import (
	v2 "github.com/moby/moby/v2/daemon/pkg/plugin/v2"
	"github.com/moby/moby/v2/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (pm *Manager) enable(_ *v2.Plugin, _ *controller, _ bool) error {
	return nil
}

func (pm *Manager) initSpec(_ *v2.Plugin) (*specs.Spec, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "Manager.initSpec"}
}

func (pm *Manager) disable(_ *v2.Plugin, _ *controller) error {
	return nil
}

func (pm *Manager) restore(_ *v2.Plugin, _ *controller) error {
	return nil
}

func (pm *Manager) Shutdown() {}

func recursiveUnmount(_ string) error {
	return nil
}
