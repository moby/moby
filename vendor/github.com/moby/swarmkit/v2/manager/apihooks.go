package manager

import (
	"context"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

func (m *Manager) networkAllocator() networkallocator.NetworkAllocator {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.allocator == nil {
		return nil
	}
	return m.allocator.NetworkAllocator()
}

func (m *Manager) OnGetNetwork(ctx context.Context, n *api.Network, appdataTypeURL string, appdata []byte) error {
	if nwh, ok := m.networkAllocator().(networkallocator.OnGetNetworker); ok {
		return nwh.OnGetNetwork(ctx, n, appdataTypeURL, appdata)
	}
	return nil
}

func (m *Manager) OnListNetworks(ctx context.Context, networks []*api.Network, appdataTypeURL string, appdata []byte) error {
	if nwh, ok := m.networkAllocator().(networkallocator.OnGetNetworker); ok {
		for _, n := range networks {
			if err := nwh.OnGetNetwork(ctx, n, appdataTypeURL, appdata); err != nil {
				return err
			}
		}
	}
	return nil
}
