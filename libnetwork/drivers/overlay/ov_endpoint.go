//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/ns"
)

type endpointTable map[string]*endpoint

type endpoint struct {
	id     string
	nid    string
	ifName string
	mac    net.HardwareAddr
	addr   netip.Prefix
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]interface{}) error {
	var err error
	if err = validateID(nid, eid); err != nil {
		return err
	}

	// Since we perform lazy configuration make sure we try
	// configuring the driver when we enter CreateEndpoint since
	// CreateNetwork may not be called in every node.
	if err := d.configure(); err != nil {
		return err
	}

	n := d.lockNetwork(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}
	defer n.mu.Unlock()

	ep := &endpoint{
		id:  eid,
		nid: n.id,
		mac: ifInfo.MacAddress(),
	}
	var ok bool
	ep.addr, ok = netiputil.ToPrefix(ifInfo.Address())
	if !ok {
		return fmt.Errorf("create endpoint was not passed interface IP address")
	}

	if s := n.getSubnetforIP(ep.addr); s == nil {
		return fmt.Errorf("no matching subnet for IP %q in network %q", ep.addr, nid)
	}

	if ep.mac == nil {
		ep.mac = netutils.GenerateMACFromIP(ep.addr.Addr().AsSlice())
		if err := ifInfo.SetMacAddress(ep.mac); err != nil {
			return err
		}
	}

	n.endpoints[ep.id] = ep

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	nlh := ns.NlHandle()

	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.lockNetwork(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}
	defer n.mu.Unlock()

	ep := n.endpoints[eid]
	if ep == nil {
		return fmt.Errorf("endpoint id %q not found", eid)
	}

	delete(n.endpoints, eid)

	if ep.ifName == "" {
		return nil
	}

	link, err := nlh.LinkByName(ep.ifName)
	if err != nil {
		log.G(context.TODO()).Debugf("Failed to retrieve interface (%s)'s link on endpoint (%s) delete: %v", ep.ifName, ep.id, err)
		return nil
	}
	if err := nlh.LinkDel(link); err != nil {
		log.G(context.TODO()).Debugf("Failed to delete interface (%s)'s link on endpoint (%s) delete: %v", ep.ifName, ep.id, err)
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}
