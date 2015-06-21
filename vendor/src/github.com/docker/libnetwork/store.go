package libnetwork

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
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

	nws, err := c.getNetworksFromStore()
	if err == nil {
		c.processNetworkUpdate(nws, nil)
	} else if err != datastore.ErrKeyNotFound {
		log.Warnf("failed to read networks from datastore during init : %v", err)
	}
	return c.watchNetworks()
}

func (c *controller) getNetworksFromStore() ([]*store.KVPair, error) {
	c.Lock()
	cs := c.store
	c.Unlock()
	return cs.KVStore().List(datastore.Key(datastore.NetworkKeyPrefix))
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

func (c *controller) watchNetworks() error {
	if !c.validateDatastoreConfig() {
		return nil
	}

	c.Lock()
	cs := c.store
	c.Unlock()

	nwPairs, err := cs.KVStore().WatchTree(datastore.Key(datastore.NetworkKeyPrefix), nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case nws := <-nwPairs:
				c.Lock()
				tmpview := networkTable{}
				lview := c.networks
				c.Unlock()
				for k, v := range lview {
					global, _ := v.isGlobalScoped()
					if global {
						tmpview[k] = v
					}
				}
				c.processNetworkUpdate(nws, &tmpview)
				// Delete processing
				for k := range tmpview {
					c.Lock()
					existing, ok := c.networks[k]
					c.Unlock()
					if !ok {
						continue
					}
					tmp := network{}
					if err := c.store.GetObject(datastore.Key(existing.Key()...), &tmp); err != datastore.ErrKeyNotFound {
						continue
					}
					if err := existing.deleteNetwork(); err != nil {
						log.Debugf("Delete failed %s: %s", existing.name, err)
					}
				}
			}
		}
	}()
	return nil
}

func (n *network) watchEndpoints() error {
	if !n.ctrlr.validateDatastoreConfig() {
		return nil
	}

	n.Lock()
	cs := n.ctrlr.store
	tmp := endpoint{network: n}
	n.stopWatchCh = make(chan struct{})
	stopCh := n.stopWatchCh
	n.Unlock()

	epPairs, err := cs.KVStore().WatchTree(datastore.Key(tmp.KeyPrefix()...), stopCh)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case eps := <-epPairs:
				n.Lock()
				tmpview := endpointTable{}
				lview := n.endpoints
				n.Unlock()
				for k, v := range lview {
					global, _ := v.network.isGlobalScoped()
					if global {
						tmpview[k] = v
					}
				}
				for _, epe := range eps {
					var ep endpoint
					err := json.Unmarshal(epe.Value, &ep)
					if err != nil {
						log.Error(err)
						continue
					}
					delete(tmpview, ep.id)
					ep.dbIndex = epe.LastIndex
					ep.network = n
					if n.ctrlr.processEndpointUpdate(&ep) {
						err = n.ctrlr.newEndpointFromStore(epe.Key, &ep)
						if err != nil {
							log.Error(err)
						}
					}
				}
				// Delete processing
				for k := range tmpview {
					n.Lock()
					existing, ok := n.endpoints[k]
					n.Unlock()
					if !ok {
						continue
					}
					tmp := endpoint{}
					if err := cs.GetObject(datastore.Key(existing.Key()...), &tmp); err != datastore.ErrKeyNotFound {
						continue
					}
					if err := existing.deleteEndpoint(); err != nil {
						log.Debugf("Delete failed %s: %s", existing.name, err)
					}
				}
			}
		}
	}()
	return nil
}

func (n *network) stopWatch() {
	n.Lock()
	if n.stopWatchCh != nil {
		close(n.stopWatchCh)
		n.stopWatchCh = nil
	}
	n.Unlock()
}

func (c *controller) processNetworkUpdate(nws []*store.KVPair, prune *networkTable) {
	for _, kve := range nws {
		var n network
		err := json.Unmarshal(kve.Value, &n)
		if err != nil {
			log.Error(err)
			continue
		}
		if prune != nil {
			delete(*prune, n.id)
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
