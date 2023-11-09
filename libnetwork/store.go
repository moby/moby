package libnetwork

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/scope"
)

func (c *Controller) initStores() error {
	if c.cfg == nil {
		return nil
	}
	var err error
	c.store, err = datastore.New(c.cfg.Scope)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) closeStores() {
	if store := c.store; store != nil {
		store.Close()
	}
}

func (c *Controller) getStore() *datastore.Store {
	return c.store
}

func (c *Controller) getNetworkFromStore(nid string) (*Network, error) {
	for _, n := range c.getNetworksFromStore() {
		if n.id == nid {
			return n, nil
		}
	}
	return nil, ErrNoSuchNetwork(nid)
}

func (c *Controller) getNetworks() ([]*Network, error) {
	var nl []*Network

	store := c.getStore()
	if store == nil {
		return nil, nil
	}

	kvol, err := store.List(datastore.Key(datastore.NetworkKeyPrefix),
		&Network{ctrlr: c})
	if err != nil && err != datastore.ErrKeyNotFound {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	for _, kvo := range kvol {
		n := kvo.(*Network)
		n.ctrlr = c

		ec := &endpointCnt{n: n}
		err = store.GetObject(datastore.Key(ec.Key()...), ec)
		if err != nil && !n.inDelete {
			log.G(context.TODO()).Warnf("Could not find endpoint count key %s for network %s while listing: %v", datastore.Key(ec.Key()...), n.Name(), err)
			continue
		}

		n.epCnt = ec
		if n.scope == "" {
			n.scope = scope.Local
		}
		nl = append(nl, n)
	}

	return nl, nil
}

func (c *Controller) getNetworksFromStore() []*Network { // FIXME: unify with c.getNetworks()
	var nl []*Network

	store := c.getStore()
	kvol, err := store.List(datastore.Key(datastore.NetworkKeyPrefix), &Network{ctrlr: c})
	if err != nil {
		if err != datastore.ErrKeyNotFound {
			log.G(context.TODO()).Debugf("failed to get networks from store: %v", err)
		}
		return nil
	}

	kvep, err := store.Map(datastore.Key(epCntKeyPrefix), &endpointCnt{})
	if err != nil && err != datastore.ErrKeyNotFound {
		log.G(context.TODO()).Warnf("failed to get endpoint_count map from store: %v", err)
	}

	for _, kvo := range kvol {
		n := kvo.(*Network)
		n.mu.Lock()
		n.ctrlr = c
		ec := &endpointCnt{n: n}
		// Trim the leading & trailing "/" to make it consistent across all stores
		if val, ok := kvep[strings.Trim(datastore.Key(ec.Key()...), "/")]; ok {
			ec = val.(*endpointCnt)
			ec.n = n
			n.epCnt = ec
		}
		if n.scope == "" {
			n.scope = scope.Local
		}
		n.mu.Unlock()
		nl = append(nl, n)
	}

	return nl
}

func (n *Network) getEndpointFromStore(eid string) (*Endpoint, error) {
	store := n.ctrlr.getStore()
	ep := &Endpoint{id: eid, network: n}
	err := store.GetObject(datastore.Key(ep.Key()...), ep)
	if err != nil {
		return nil, fmt.Errorf("could not find endpoint %s: %w", eid, err)
	}
	return ep, nil
}

func (n *Network) getEndpointsFromStore() ([]*Endpoint, error) {
	var epl []*Endpoint

	tmp := Endpoint{network: n}
	store := n.getController().getStore()
	kvol, err := store.List(datastore.Key(tmp.KeyPrefix()...), &Endpoint{network: n})
	if err != nil {
		if err != datastore.ErrKeyNotFound {
			return nil, fmt.Errorf("failed to get endpoints for network %s: %w",
				n.Name(), err)
		}
		return nil, nil
	}

	for _, kvo := range kvol {
		ep := kvo.(*Endpoint)
		epl = append(epl, ep)
	}

	return epl, nil
}

func (c *Controller) updateToStore(kvObject datastore.KVObject) error {
	cs := c.getStore()
	if cs == nil {
		return fmt.Errorf("datastore is not initialized")
	}

	if err := cs.PutObjectAtomic(kvObject); err != nil {
		if err == datastore.ErrKeyModified {
			return err
		}
		return fmt.Errorf("failed to update store for object type %T: %v", kvObject, err)
	}

	return nil
}

func (c *Controller) deleteFromStore(kvObject datastore.KVObject) error {
	cs := c.getStore()
	if cs == nil {
		return fmt.Errorf("datastore is not initialized")
	}

retry:
	if err := cs.DeleteObjectAtomic(kvObject); err != nil {
		if err == datastore.ErrKeyModified {
			if err := cs.GetObject(datastore.Key(kvObject.Key()...), kvObject); err != nil {
				return fmt.Errorf("could not update the kvobject to latest when trying to delete: %v", err)
			}
			log.G(context.TODO()).Warnf("Error (%v) deleting object %v, retrying....", err, kvObject.Key())
			goto retry
		}
		return err
	}

	return nil
}

func (c *Controller) networkCleanup() {
	for _, n := range c.getNetworksFromStore() {
		if n.inDelete {
			log.G(context.TODO()).Infof("Removing stale network %s (%s)", n.Name(), n.ID())
			if err := n.delete(true, true); err != nil {
				log.G(context.TODO()).Debugf("Error while removing stale network: %v", err)
			}
		}
	}
}
