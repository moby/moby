package libnetwork

import "context"

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
