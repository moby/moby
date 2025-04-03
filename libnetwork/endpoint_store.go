// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package libnetwork

import (
	"context"

	"github.com/docker/docker/internal/maputil"
)

// storeEndpoint inserts or updates the endpoint in the store and the in-memory
// cache maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) storeEndpoint(ctx context.Context, ep *Endpoint) error {
	if err := c.updateToStore(ctx, ep); err != nil {
		return err
	}
	c.cacheEndpoint(ep)
	return nil
}

// deleteStoredEndpoint deletes the endpoint from the store and the in-memory
// cache maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) deleteStoredEndpoint(ep *Endpoint) error {
	if err := c.deleteFromStore(ep); err != nil {
		return err
	}

	c.endpointsMu.Lock()
	defer c.endpointsMu.Unlock()
	delete(c.endpoints, ep.id)

	return nil
}

// cacheEndpoint caches the endpoint in the in-memory cache of endpoints
// maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) cacheEndpoint(ep *Endpoint) {
	c.endpointsMu.Lock()
	defer c.endpointsMu.Unlock()
	c.endpoints[ep.id] = ep
}

// findEndpoints looks for all endpoints matching the filter from the in-memory
// cache of endpoints maintained by the Controller.
//
// This method is thread-safe, but do not use it unless you're sure your code
// uses the returned endpoints in thread-safe way (see the comment on
// Controller.endpoints).
func (c *Controller) findEndpoints(filter func(ep *Endpoint) bool) []*Endpoint {
	c.endpointsMu.Lock()
	defer c.endpointsMu.Unlock()
	return maputil.FilterValues(c.endpoints, filter)
}

func filterEndpointByNetworkId(expected string) func(ep *Endpoint) bool {
	return func(ep *Endpoint) bool {
		return ep.network != nil && ep.network.id == expected
	}
}
