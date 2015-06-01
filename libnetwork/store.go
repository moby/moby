package libnetwork

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

func (c *controller) initDataStore() error {
	if c.cfg == nil {
		return fmt.Errorf("datastore initialization requires a valid configuration")
	}

	store, err := datastore.NewDataStore(&c.cfg.Datastore)
	if err != nil {
		return err
	}
	c.Lock()
	c.store = store
	c.Unlock()
	return c.watchStore()
}

func (c *controller) newNetworkFromStore(n *network) error {
	c.Lock()
	n.ctrlr = c
	c.Unlock()
	n.endpoints = endpointTable{}

	return c.addNetwork(n)
}

func (c *controller) addNetworkToStore(n *network) error {
	if isReservedNetwork(n.Name()) {
		return nil
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. Network %s is not added to the store", n.Name())
		return nil
	}

	// Commenting out AtomicPut due to https://github.com/docker/swarm/issues/875,
	// Also Network object is Keyed with UUID & hence an Atomic put is not mandatory.
	// return cs.PutObjectAtomic(n)

	return cs.PutObject(n)
}

func (c *controller) getNetworkFromStore(nid types.UUID) (*network, error) {
	n := network{id: nid}
	if err := c.store.GetObject(datastore.Key(n.Key()...), &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (c *controller) newEndpointFromStore(ep *endpoint) {
	c.Lock()
	n, ok := c.networks[ep.network.id]
	c.Unlock()

	if !ok {
		// Possibly the watch event for the network has not shown up yet
		// Try to get network from the store
		var err error
		n, err = c.getNetworkFromStore(ep.network.id)
		if err != nil {
			log.Warnf("Network (%s) unavailable for endpoint=%s", ep.network.id, ep.name)
			return
		}
		if err := c.newNetworkFromStore(n); err != nil {
			log.Warnf("Failed to add Network (%s - %s) from store", n.name, n.id)
			return
		}
	}

	ep.network = n
	_, err := n.EndpointByID(string(ep.id))
	if _, ok := err.(ErrNoSuchEndpoint); ok {
		n.addEndpoint(ep)
	}
}

func (c *controller) addEndpointToStore(ep *endpoint) error {
	if isReservedNetwork(ep.network.name) {
		return nil
	}
	c.Lock()
	cs := c.store
	c.Unlock()
	if cs == nil {
		log.Debugf("datastore not initialized. endpoint %s is not added to the store", ep.name)
		return nil
	}

	// Commenting out AtomicPut due to https://github.com/docker/swarm/issues/875,
	// Also Network object is Keyed with UUID & hence an Atomic put is not mandatory.
	// return cs.PutObjectAtomic(ep)

	return cs.PutObject(ep)
}

func (c *controller) getEndpointFromStore(eid types.UUID) (*endpoint, error) {
	ep := endpoint{id: eid}
	if err := c.store.GetObject(datastore.Key(ep.Key()...), &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func (c *controller) watchStore() error {
	c.Lock()
	cs := c.store
	c.Unlock()

	nwPairs, err := cs.KVStore().WatchTree(datastore.Key(datastore.NetworkKeyPrefix), c.stopChan)
	if err != nil {
		return err
	}
	epPairs, err := cs.KVStore().WatchTree(datastore.Key(datastore.EndpointKeyPrefix), c.stopChan)
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
						// Skip existing network update
						if existing.dbIndex != n.dbIndex {
							log.Debugf("Skipping network update for %s (%s)", n.name, n.id)
						}
						continue
					}

					if err = c.newNetworkFromStore(&n); err != nil {
						log.Error(err)
						continue
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
					c.Lock()
					n, ok := c.networks[ep.network.id]
					c.Unlock()
					if ok {
						existing, _ := n.EndpointByID(string(ep.id))
						if existing != nil {
							// Skip existing endpoint update
							if existing.(*endpoint).dbIndex != ep.dbIndex {
								log.Debugf("Skipping endpoint update for %s (%s)", ep.name, ep.id)
							}
							continue
						}
					}

					c.newEndpointFromStore(&ep)
				}
			}
		}
	}()
	return nil
}
