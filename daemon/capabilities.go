package daemon

import (
	"context"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/system"
)

func (d *Daemon) SystemCapabilities(ctx context.Context) (system.Capabilities, error) {
	return capManager.getCapabilities(ctx, d)
}

type capabilitiesProvider interface {
	UsesSnapshotter() bool
}

// Daemon must implement capabilitiesProvider
var _ capabilitiesProvider = &Daemon{}

var capManager = &capabilitiesManager{}

type capabilitiesManager struct {
	sync.RWMutex
	cacheValid         bool
	cachedCapabilities system.Capabilities
}

func (m *capabilitiesManager) getCapabilities(ctx context.Context, p capabilitiesProvider) (system.Capabilities, error) {
	m.RLock()
	if m.cacheValid {
		defer m.RUnlock()
		return m.cachedCapabilities, nil
	}
	m.RUnlock()

	m.Lock()
	defer m.Unlock()
	m.cachedCapabilities = m.fetchCapabilities(ctx, p)
	m.cacheValid = true
	return m.cachedCapabilities, nil
}

func (m *capabilitiesManager) invalidateCache(ctx context.Context) {
	log.G(ctx).Debug("invalidating cached system capabilities")
	m.Lock()
	defer m.Unlock()

	m.cacheValid = false
}

func (m *capabilitiesManager) fetchCapabilities(ctx context.Context, p capabilitiesProvider) system.Capabilities {
	log.G(ctx).Debug("fetching system capabilities")
	var registryClientAuth bool
	if p.UsesSnapshotter() {
		registryClientAuth = true
	}
	return system.Capabilities{
		RegistryClientAuth: registryClientAuth,
	}
}
