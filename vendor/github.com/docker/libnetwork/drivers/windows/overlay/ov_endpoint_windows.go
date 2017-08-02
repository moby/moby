package overlay

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/Microsoft/hcsshim"
	"github.com/docker/libnetwork/driverapi"
	"github.com/sirupsen/logrus"
)

type endpointTable map[string]*endpoint

const overlayEndpointPrefix = "overlay/endpoint"

type endpoint struct {
	id        string
	nid       string
	profileId string
	remote    bool
	mac       net.HardwareAddr
	addr      *net.IPNet
}

func validateID(nid, eid string) error {
	if nid == "" {
		return fmt.Errorf("invalid network id")
	}

	if eid == "" {
		return fmt.Errorf("invalid endpoint id")
	}

	return nil
}

func (n *network) endpoint(eid string) *endpoint {
	n.Lock()
	defer n.Unlock()

	return n.endpoints[eid]
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

func (n *network) removeEndpointWithAddress(addr *net.IPNet) {
	var networkEndpoint *endpoint
	n.Lock()
	for _, ep := range n.endpoints {
		if ep.addr.IP.Equal(addr.IP) {
			networkEndpoint = ep
			break
		}
	}

	if networkEndpoint != nil {
		delete(n.endpoints, networkEndpoint.id)
	}
	n.Unlock()

	if networkEndpoint != nil {
		logrus.Debugf("Removing stale endpoint from HNS")
		_, err := hcsshim.HNSEndpointRequest("DELETE", networkEndpoint.profileId, "")

		if err != nil {
			logrus.Debugf("Failed to delete stale overlay endpoint (%s) from hns", networkEndpoint.id[0:7])
		}
	}
}

func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo,
	epOptions map[string]interface{}) error {
	var err error
	if err = validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := n.endpoint(eid)
	if ep != nil {
		logrus.Debugf("Deleting stale endpoint %s", eid)
		n.deleteEndpoint(eid)

		_, err := hcsshim.HNSEndpointRequest("DELETE", ep.profileId, "")
		if err != nil {
			return err
		}
	}

	ep = &endpoint{
		id:   eid,
		nid:  n.id,
		addr: ifInfo.Address(),
		mac:  ifInfo.MacAddress(),
	}

	if ep.addr == nil {
		return fmt.Errorf("create endpoint was not passed interface IP address")
	}

	if s := n.getSubnetforIP(ep.addr); s == nil {
		return fmt.Errorf("no matching subnet for IP %q in network %q\n", ep.addr, nid)
	}

	// Todo: Add port bindings and qos policies here

	hnsEndpoint := &hcsshim.HNSEndpoint{
		Name:              eid,
		VirtualNetwork:    n.hnsId,
		IPAddress:         ep.addr.IP,
		EnableInternalDNS: true,
	}

	if ep.mac != nil {
		hnsEndpoint.MacAddress = ep.mac.String()
	}

	paPolicy, err := json.Marshal(hcsshim.PaPolicy{
		Type: "PA",
		PA:   n.providerAddress,
	})

	if err != nil {
		return err
	}

	hnsEndpoint.Policies = append(hnsEndpoint.Policies, paPolicy)

	configurationb, err := json.Marshal(hnsEndpoint)
	if err != nil {
		return err
	}

	hnsresponse, err := hcsshim.HNSEndpointRequest("POST", "", string(configurationb))
	if err != nil {
		return err
	}

	ep.profileId = hnsresponse.Id

	if ep.mac == nil {
		ep.mac, err = net.ParseMAC(hnsresponse.MacAddress)
		if err != nil {
			return err
		}

		if err := ifInfo.SetMacAddress(ep.mac); err != nil {
			return err
		}
	}

	n.addEndpoint(ep)

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("endpoint id %q not found", eid)
	}

	n.deleteEndpoint(eid)

	_, err := hcsshim.HNSEndpointRequest("DELETE", ep.profileId, "")
	if err != nil {
		return err
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	if err := validateID(nid, eid); err != nil {
		return nil, err
	}

	n := d.network(nid)
	if n == nil {
		return nil, fmt.Errorf("network id %q not found", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return nil, fmt.Errorf("endpoint id %q not found", eid)
	}

	data := make(map[string]interface{}, 1)
	data["hnsid"] = ep.profileId
	data["AllowUnqualifiedDNSQuery"] = true
	return data, nil
}
