package libnetwork

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/maputil"
)

// storeNetwork inserts or updates the network in the store and the in-memory
// cache maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) storeNetwork(ctx context.Context, n *Network) error {
	if err := c.updateToStore(ctx, n); err != nil {
		return err
	}
	c.cacheNetwork(n)
	return nil
}

// deleteStoredNetwork deletes the network from the store and the in-memory
// cache maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) deleteStoredNetwork(n *Network) error {
	if err := c.deleteFromStore(n); err != nil {
		return err
	}

	c.networksMu.Lock()
	defer c.networksMu.Unlock()
	delete(c.networks, n.id)

	return nil
}

// cacheNetwork caches the network in the in-memory cache of networks
// maintained by the Controller.
//
// This method is thread-safe.
func (c *Controller) cacheNetwork(n *Network) {
	c.networksMu.Lock()
	defer c.networksMu.Unlock()
	c.networks[n.ID()] = n
}

// findNetworks looks for all networks matching the filter from the in-memory
// cache of networks maintained by the Controller.
//
// This method is thread-safe, but do not use it unless you're sure your code
// uses the returned networks in thread-safe way (see the comment on
// Controller.networks).
func (c *Controller) findNetworks(filter func(nw *Network) bool) []*Network {
	c.networksMu.Lock()
	defer c.networksMu.Unlock()
	return maputil.FilterValues(c.networks, filter)
}

func filterNetworkByConfigFrom(expected string) func(nw *Network) bool {
	return func(nw *Network) bool {
		return nw.configFrom == expected
	}
}
