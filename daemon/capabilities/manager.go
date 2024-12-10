package capabilities

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
)

type Provider interface {
	UsesSnapshotter() bool
}

// NewManager returns a new capabilities.Manager.
// Use this instead of Manager{}.
func NewManager() Manager {
	cond := sync.NewCond(&sync.Mutex{})
	return Manager{
		stateCond: cond,
	}
}

type Manager struct {
	// cacheReady is  not guarded/an atomic type
	// so that InvalidateCache is fast/doesn't have
	// to contend with GetCapabilities.
	cacheReady atomic.Bool

	stateCond         *sync.Cond
	refreshInProgress bool              // guarded by stateCond
	cache             capabilitiesCache // guarded by stateCond
}

// GetCapabilities returns Capabilities according to the requested version.
// If the requested version N <= CurrentVersion, the requested version is
// returned. Otherwise, GetCapabilities returns the newest capabilities
// version (CurrentVersion).
// Capabilities are cached by the manager, so as long as the cache is valid
// (and the cache isn't currently being refreshed) GetCapabilities should be
// quick and have little performance impact. However, changes to daemon
// configuration/reloads can invalidate the cache. If this happens, the first
// call to GetCapabilities will launch a goroutine to refresh the cache.
// Subsequent calls to GetCapabilities will block until the cache has been
// refreshed.
func (m *Manager) GetCapabilities(ctx context.Context, p Provider, requestedVersion int) (VersionedCapabilities, error) {
	negotiatedVersion := negotiatedCapabilitiesVersion(requestedVersion)

	// even though cacheReady is a thread-safe type, grab stateCond before
	// checking it, otherwise we'd introduce a race between checking cacheReady
	// and updating refreshInProgress (since we'd need to acquire stateCond to)
	// update refreshInProgress
	m.stateCond.L.Lock()
	defer m.stateCond.L.Unlock()
	// if many threads try to call GetCapabilities while
	// the cache is dirty, only one should refresh
	if shouldRefresh := m.cacheReady.CompareAndSwap(false, true); shouldRefresh {
		// tell other callers that we're going to refresh and they should wait
		m.refreshInProgress = true

		go func() {
			newCapabilities := m.fetchCapabilities(ctx, p)

			m.stateCond.L.Lock()
			m.cache = newCapabilities
			m.refreshInProgress = false
			m.stateCond.L.Unlock()
			// tell all waiting callers that we're done
			m.stateCond.Broadcast()
		}()
	}

	// wait for any refreshes to be finished
	// before accessing cache
	for m.refreshInProgress {
		m.stateCond.Wait()
	}

	switch negotiatedVersion {
	case V1:
		return m.cache.v1, nil
	}

	return nil, errdefs.System(errors.New("TODO"))
}

// InvalidateCache causes the capabilities cache to be refetched.
// It should be called whenever daemon configurations change, in
// order to make sure that capabilities returned by GetCapabilities
// are accurate to the current state of the daemon.
// InvalidateCache is not blocking – it does not wait until the cache
// is rebuilt, just marks it "dirty" – rebuilding happens on the
// next GetCapabilities call.
func (m *Manager) InvalidateCache(ctx context.Context) {
	log.G(ctx).Debug("invalidating cached system capabilities")
	m.cacheReady.Store(false)
}

func (m *Manager) fetchCapabilities(ctx context.Context, p Provider) capabilitiesCache {
	log.G(ctx).Debug("fetching system capabilities")

	v1 := capabilitiesV1{
		CapabilitiesBase: CapabilitiesBase{
			CapabilitiesVersion: 1,
		},
	}
	if p.UsesSnapshotter() {
		v1.RegistryClientAuth = true
	}
	return capabilitiesCache{
		v1: v1,
	}
}

type capabilitiesCache struct {
	v1 capabilitiesV1
}
