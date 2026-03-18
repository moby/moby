//go:build linux

package ipvlan

import (
	"context"
	"errors"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func (d *driver) network(nid string) *network {
	d.mu.Lock()
	n, ok := d.networks[nid]
	d.mu.Unlock()
	if !ok {
		log.G(context.TODO()).Errorf("network id %s not found", nid)
	}

	return n
}

func (d *driver) addNetwork(n *network) {
	d.mu.Lock()
	d.networks[n.id] = n
	d.mu.Unlock()
}

func (d *driver) deleteNetwork(nid string) {
	d.mu.Lock()
	delete(d.networks, nid)
	d.mu.Unlock()
}

// getNetworks Safely returns a slice of existing networks
func (d *driver) getNetworks() []*network {
	d.mu.Lock()
	defer d.mu.Unlock()

	ls := make([]*network, 0, len(d.networks))
	for _, nw := range d.networks {
		ls = append(ls, nw)
	}

	return ls
}

func (n *network) endpoint(eid string) (*endpoint, error) {
	if eid == "" {
		return nil, errors.New("invalid endpoint id")
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	ep, ok := n.endpoints[eid]
	if !ok || ep == nil {
		return nil, errors.New("could not find endpoint with id " + eid)
	}
	return ep, nil
}

func (n *network) addEndpoint(ep *endpoint) {
	n.mu.Lock()
	n.endpoints[ep.id] = ep
	n.mu.Unlock()
}

func (n *network) deleteEndpoint(eid string) {
	n.mu.Lock()
	delete(n.endpoints, eid)
	n.mu.Unlock()
}

func validateID(nid, eid string) error {
	if nid == "" {
		return errors.New("invalid network id")
	}
	if eid == "" {
		return errors.New("invalid endpoint id")
	}

	return nil
}

func (d *driver) getNetwork(id string) (*network, error) {
	if id == "" {
		return nil, types.InvalidParameterErrorf("invalid network id")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	nw, ok := d.networks[id]
	if !ok || nw == nil {
		return nil, types.NotFoundErrorf("network not found: %s", id)
	}
	return nw, nil
}
