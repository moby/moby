package libnetwork

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

func (c *controller) validateDatastoreConfig() bool {
	if c.cfg == nil || c.cfg.Datastore.Client.Provider == "" || c.cfg.Datastore.Client.Address == "" {
		return false
	}
	return true
}

func (c *controller) initDataStore() error {
	c.Lock()
	cfg := c.cfg
	c.Unlock()
	if !c.validateDatastoreConfig() {
		return fmt.Errorf("datastore initialization requires a valid configuration")
	}

	store, err := datastore.NewDataStore(&cfg.Datastore)
	if err != nil {
		return err
	}
	c.Lock()
	c.store = store
	c.Unlock()
	return c.watchStore()
}

func (c *controller) newNetworkFromStore(n *network) error {
	n.Lock()
	n.ctrlr = c
	n.endpoints = endpointTable{}
	n.Unlock()

	return c.addNetwork(n)
}

func (c *controller) updateNetworkToStore(n *network) error {
	global, err := n.isGlobalScoped()
	if err != nil || !global {
		return err
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. Network %s is not added to the store", n.Name())
		return nil
	}

	return cs.PutObjectAtomic(n)
}

func (c *controller) deleteNetworkFromStore(n *network) error {
	global, err := n.isGlobalScoped()
	if err != nil || !global {
		return err
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. Network %s is not deleted from datastore", n.Name())
		return nil
	}

	if err := cs.DeleteObjectAtomic(n); err != nil {
		return err
	}

	return nil
}

func (c *controller) getNetworkFromStore(nid types.UUID) (*network, error) {
	n := network{id: nid}
	if err := c.store.GetObject(datastore.Key(n.Key()...), &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (c *controller) newEndpointFromStore(key string, ep *endpoint) error {
	ep.Lock()
	n := ep.network
	id := ep.id
	ep.Unlock()
	if n == nil {
		// Possibly the watch event for the network has not shown up yet
		// Try to get network from the store
		nid, err := networkIDFromEndpointKey(key, ep)
		if err != nil {
			return err
		}
		n, err = c.getNetworkFromStore(nid)
		if err != nil {
			return err
		}
		if err := c.newNetworkFromStore(n); err != nil {
			return err
		}
		n = c.networks[nid]
	}

	_, err := n.EndpointByID(string(id))
	if err != nil {
		if _, ok := err.(ErrNoSuchEndpoint); ok {
			return n.addEndpoint(ep)
		}
	}
	return err
}

func (c *controller) updateEndpointToStore(ep *endpoint) error {
	ep.Lock()
	n := ep.network
	name := ep.name
	ep.Unlock()
	global, err := n.isGlobalScoped()
	if err != nil || !global {
		return err
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. endpoint %s is not added to the store", name)
		return nil
	}

	return cs.PutObjectAtomic(ep)
}

func (c *controller) getEndpointFromStore(eid types.UUID) (*endpoint, error) {
	ep := endpoint{id: eid}
	if err := c.store.GetObject(datastore.Key(ep.Key()...), &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func (c *controller) deleteEndpointFromStore(ep *endpoint) error {
	ep.Lock()
	n := ep.network
	ep.Unlock()
	global, err := n.isGlobalScoped()
	if err != nil || !global {
		return err
	}

	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. endpoint %s is not deleted from datastore", ep.Name())
		return nil
	}

	if err := cs.DeleteObjectAtomic(ep); err != nil {
		return err
	}

	return nil
}

func (c *controller) watchStore() error {
	c.Lock()
	cs := c.store
	c.Unlock()

	nwPairs, err := cs.KVStore().WatchTree(datastore.Key(datastore.NetworkKeyPrefix), nil)
	if err != nil {
		return err
	}
	epPairs, err := cs.KVStore().WatchTree(datastore.Key(datastore.EndpointKeyPrefix), nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case nws := <-nwPairs:
				for _, kve := range nws {
					var n network
					err := json.Unmarshal(kve.Value, &n)
					if err != nil {
						log.Error(err)
						continue
					}
					n.dbIndex = kve.LastIndex
					c.Lock()
					existing, ok := c.networks[n.id]
					c.Unlock()
					if ok {
						existing.Lock()
						// Skip existing network update
						if existing.dbIndex != n.dbIndex {
							existing.dbIndex = n.dbIndex
							existing.endpointCnt = n.endpointCnt
						}
						existing.Unlock()
						continue
					}

					if err = c.newNetworkFromStore(&n); err != nil {
						log.Error(err)
					}
				}
			case eps := <-epPairs:
				for _, epe := range eps {
					var ep endpoint
					err := json.Unmarshal(epe.Value, &ep)
					if err != nil {
						log.Error(err)
						continue
					}
					ep.dbIndex = epe.LastIndex
					n, err := c.networkFromEndpointKey(epe.Key, &ep)
					if err != nil {
						if _, ok := err.(ErrNoSuchNetwork); !ok {
							log.Error(err)
							continue
						}
					}
					if n != nil {
						ep.network = n.(*network)
					}
					if c.processEndpointUpdate(&ep) {
						err = c.newEndpointFromStore(epe.Key, &ep)
						if err != nil {
							log.Error(err)
						}
					}
				}
			}
		}
	}()
	return nil
}

func (c *controller) networkFromEndpointKey(key string, ep *endpoint) (Network, error) {
	nid, err := networkIDFromEndpointKey(key, ep)
	if err != nil {
		return nil, err
	}
	return c.NetworkByID(string(nid))
}

func networkIDFromEndpointKey(key string, ep *endpoint) (types.UUID, error) {
	eKey, err := datastore.ParseKey(key)
	if err != nil {
		return types.UUID(""), err
	}
	return ep.networkIDFromKey(eKey)
}

func (c *controller) processEndpointUpdate(ep *endpoint) bool {
	nw := ep.network
	if nw == nil {
		return true
	}
	nw.Lock()
	id := nw.id
	nw.Unlock()

	c.Lock()
	n, ok := c.networks[id]
	c.Unlock()
	if !ok {
		return true
	}
	existing, _ := n.EndpointByID(string(ep.id))
	if existing == nil {
		return true
	}

	ee := existing.(*endpoint)
	ee.Lock()
	if ee.dbIndex != ep.dbIndex {
		ee.dbIndex = ep.dbIndex
		if ee.container != nil && ep.container != nil {
			// we care only about the container id
			ee.container.id = ep.container.id
		} else {
			// we still care only about the container id, but this is a short-cut to communicate join or leave operation
			ee.container = ep.container
		}
	}
	ee.Unlock()

	return false
}
