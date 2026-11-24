package libnetwork

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/memstore"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEndpointStore(t *testing.T) {
	c, err := New(t.Context())
	assert.NilError(t, err)

	// Insert a first endpoint
	nw := &Network{id: "testNetwork"}
	ep1 := &Endpoint{network: nw, id: "testEndpoint1"}
	err = c.storeEndpoint(t.Context(), ep1)
	assert.NilError(t, err)

	// Then a second endpoint
	ep2 := &Endpoint{network: nw, id: "testEndpoint2"}
	err = c.storeEndpoint(t.Context(), ep2)
	assert.NilError(t, err)

	// Check that we can find both endpoints, and that the returned values are
	// not copies of the original ones.
	found := c.findEndpoints(filterEndpointByNetworkId("testNetwork"))
	slices.SortFunc(found, func(a, b *Endpoint) int { return strings.Compare(a.id, b.id) })
	assert.Equal(t, len(found), 2)
	assert.Check(t, is.Equal(found[0], ep1), "got: %s; expected: %s", found[0].id, ep1.id)
	assert.Check(t, is.Equal(found[1], ep2), "got: %s; expected: %s", found[1].id, ep1.id)

	// Delete the first endpoint
	err = c.deleteStoredEndpoint(ep1)
	assert.NilError(t, err)

	// Check that we can only find the second endpoint
	found = c.findEndpoints(filterEndpointByNetworkId("testNetwork"))
	assert.Equal(t, len(found), 1)
	assert.Check(t, is.Equal(found[0], ep2), "got: %s; expected: %s", found[0].id, ep2.id)

	// Store the second endpoint again
	err = c.storeEndpoint(t.Context(), ep2)
	assert.NilError(t, err)
}

// Reading an endpoint from the store must not change the in-memory cache.
func TestEndpointStoreReadDoesNotCache(t *testing.T) {
	c, err := New(t.Context())
	assert.NilError(t, err)
	nw := &Network{id: "testNetwork", controller: c}
	ep := &Endpoint{network: nw, id: "testEndpoint"}
	assert.NilError(t, c.storeEndpoint(t.Context(), ep))

	// getEndpointFromStore returns a copy, which must not replace the
	// cached instance.
	got, err := nw.getEndpointFromStore(ep.id)
	assert.NilError(t, err)
	assert.Check(t, got != ep)
	found := c.findEndpoints(filterEndpointByNetworkId(nw.id))
	assert.Assert(t, is.Len(found, 1))
	assert.Check(t, found[0] == ep)

	// An endpoint read before its deletion must not come back in the cache.
	epl, err := nw.getEndpointsFromStore()
	assert.NilError(t, err)
	assert.Check(t, is.Len(epl, 1))
	assert.NilError(t, c.deleteStoredEndpoint(ep))
	_, err = nw.getEndpointFromStore(ep.id)
	assert.Check(t, err != nil)
	assert.Check(t, is.Len(c.findEndpoints(filterEndpointByNetworkId(nw.id)), 0))
}

// An endpoint read from the store just before its deletion must not end up in
// the cache after it: Network.delete would see an active endpoint forever.
func TestEndpointStoreDeleteRacingRead(t *testing.T) {
	c, err := New(t.Context())
	assert.NilError(t, err)
	nw := &Network{id: "testNetwork", controller: c}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_, err := nw.getEndpointsFromStore()
					assert.Check(t, err)
				}
			}
		})
	}

	for i := range 2000 {
		ep := &Endpoint{network: nw, id: fmt.Sprintf("testEndpoint%d", i)}
		assert.NilError(t, c.storeEndpoint(t.Context(), ep))
		assert.NilError(t, c.deleteStoredEndpoint(ep))
	}
	close(stop)
	readers.Wait()

	assert.Check(t, is.Len(c.findEndpoints(filterEndpointByNetworkId(nw.id)), 0))
}

// A new Controller fills its caches from the store it is given.
func TestNewLoadsCaches(t *testing.T) {
	kv := memstore.New()
	c, err := New(t.Context(), config.WithKVStore(kv))
	assert.NilError(t, err)
	// New deletes endpoints that are in no sandbox, except on config-only
	// networks, so store the endpoint on one.
	nw := &Network{id: "testNetwork", controller: c, persist: true, configOnly: true}
	ep := &Endpoint{network: nw, id: "testEndpoint"}
	assert.NilError(t, c.storeNetwork(t.Context(), nw))
	assert.NilError(t, c.storeEndpoint(t.Context(), ep))

	c, err = New(t.Context(), config.WithKVStore(kv))
	assert.NilError(t, err)
	nws := c.findNetworks(func(n *Network) bool { return n.id == nw.id })
	assert.Check(t, is.Len(nws, 1))
	eps := c.findEndpoints(filterEndpointByNetworkId(nw.id))
	assert.Assert(t, is.Len(eps, 1))
	assert.Check(t, is.Equal(eps[0].id, ep.id))
}
