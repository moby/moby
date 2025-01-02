package capabilities

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/system"
)

type Provider interface {
	UsesSnapshotter() bool
}

// NewManager returns a new capabilities.Manager.
// Use this instead of Manager{}.
func NewManager(p Provider) Manager {
	cond := sync.NewCond(&sync.Mutex{})
	return Manager{
		provider:  p,
		stateCond: cond,
	}
}

type Manager struct {
	provider Provider

	// cacheReady is not guarded/an atomic type
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
func (m *Manager) GetCapabilities(ctx context.Context, requestedVersion int) (system.Capabilities, error) {
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
			newCapabilities := fetchCapabilities(ctx, m.provider)

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
	case 1:
		// 1 is the latest version, and there are no older
		// versions, so regardless of what is requested,
		// return v1
		fallthrough
	default:
		return system.Capabilities{
			Version: 1,
			Data:    m.cache.v1,
		}, nil
	}
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

func fetchCapabilities(ctx context.Context, p Provider) capabilitiesCache {
	log.G(ctx).Debug("fetching system capabilities")

	v1 := map[string]any{}
	v1["registry-client-auth"] = p.UsesSnapshotter()
	return capabilitiesCache{
		v1: v1,
	}
}

type capabilitiesCache struct {
	v1 map[string]any
}

func negotiatedCapabilitiesVersion(requestVersion int) int {
	if requestVersion < system.CurrentVersion && requestVersion > 0 {
		return requestVersion
	}

	return system.CurrentVersion
}
