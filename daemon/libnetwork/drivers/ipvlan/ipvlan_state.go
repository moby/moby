//go:build linux

package ipvlan

import (
	"context"
	"errors"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func (d *driver) network(nid string) *network {
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()
	if !ok {
		log.G(context.TODO()).Errorf("network id %s not found", nid)
	}

	return n
}

func (d *driver) addNetwork(n *network) {
	d.Lock()
	d.networks[n.id] = n
	d.Unlock()
}

func (d *driver) deleteNetwork(nid string) {
	d.Lock()
	delete(d.networks, nid)
	d.Unlock()
}

// getNetworks Safely returns a slice of existing networks
func (d *driver) getNetworks() []*network {
	d.Lock()
	defer d.Unlock()

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
	n.Lock()
	defer n.Unlock()

	ep, ok := n.endpoints[eid]
	if !ok || ep == nil {
		return nil, errors.New("could not find endpoint with id " + eid)
	}
	return ep, nil
}

func (n *network) addEndpoint(ep *endpoint) {
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
}

func (n *network) deleteEndpoint(eid string) {
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()
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
	d.Lock()
	defer d.Unlock()
	if id == "" {
		return nil, types.InvalidParameterErrorf("invalid network id: %s", id)
	}

	if nw, ok := d.networks[id]; ok {
		return nw, nil
	}

	return nil, types.NotFoundErrorf("network not found: %s", id)
}
