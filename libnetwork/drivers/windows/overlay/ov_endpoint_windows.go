package overlay

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/windows"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
)

type endpointTable map[string]*endpoint

const overlayEndpointPrefix = "overlay/endpoint"

type endpoint struct {
	id             string
	nid            string
	profileID      string
	remote         bool
	mac            net.HardwareAddr
	addr           *net.IPNet
	disablegateway bool
	portMapping    []types.PortBinding // Operation port bindings
}

var (
	//Server 2016 (RS1) does not support concurrent add/delete of endpoints.  Therefore, we need
	//to use this mutex and serialize the add/delete of endpoints on RS1.
	endpointMu   sync.Mutex
	windowsBuild = osversion.Build()
)

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
		_, err := endpointRequest("DELETE", networkEndpoint.profileID, "")
		if err != nil {
			logrus.Debugf("Failed to delete stale overlay endpoint (%.7s) from hns", networkEndpoint.id)
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
		_, err := endpointRequest("DELETE", ep.profileID, "")
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

	s := n.getSubnetforIP(ep.addr)
	if s == nil {
		return fmt.Errorf("no matching subnet for IP %q in network %q", ep.addr, nid)
	}

	// Todo: Add port bindings and qos policies here

	hnsEndpoint := &hcsshim.HNSEndpoint{
		Name:              eid,
		VirtualNetwork:    n.hnsID,
		IPAddress:         ep.addr.IP,
		EnableInternalDNS: true,
		GatewayAddress:    s.gwIP.String(),
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

	if osversion.Build() > 16236 {
		natPolicy, err := json.Marshal(hcsshim.PaPolicy{
			Type: "OutBoundNAT",
		})

		if err != nil {
			return err
		}

		hnsEndpoint.Policies = append(hnsEndpoint.Policies, natPolicy)

		epConnectivity, err := windows.ParseEndpointConnectivity(epOptions)
		if err != nil {
			return err
		}

		ep.portMapping = epConnectivity.PortBindings
		ep.portMapping, err = windows.AllocatePorts(n.portMapper, ep.portMapping, ep.addr.IP)
		if err != nil {
			return err
		}

		defer func() {
			if err != nil {
				windows.ReleasePorts(n.portMapper, ep.portMapping)
			}
		}()

		pbPolicy, err := windows.ConvertPortBindings(ep.portMapping)
		if err != nil {
			return err
		}
		hnsEndpoint.Policies = append(hnsEndpoint.Policies, pbPolicy...)

		ep.disablegateway = true
	}

	configurationb, err := json.Marshal(hnsEndpoint)
	if err != nil {
		return err
	}

	hnsresponse, err := endpointRequest("POST", "", string(configurationb))
	if err != nil {
		return err
	}

	ep.profileID = hnsresponse.Id

	if ep.mac == nil {
		ep.mac, err = net.ParseMAC(hnsresponse.MacAddress)
		if err != nil {
			return err
		}

		if err := ifInfo.SetMacAddress(ep.mac); err != nil {
			return err
		}
	}

	ep.portMapping, err = windows.ParsePortBindingPolicies(hnsresponse.Policies)
	if err != nil {
		endpointRequest("DELETE", hnsresponse.Id, "")
		return err
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

	windows.ReleasePorts(n.portMapper, ep.portMapping)

	n.deleteEndpoint(eid)

	_, err := endpointRequest("DELETE", ep.profileID, "")
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
	data["hnsid"] = ep.profileID
	data["AllowUnqualifiedDNSQuery"] = true

	if ep.portMapping != nil {
		// Return a copy of the operational data
		pmc := make([]types.PortBinding, 0, len(ep.portMapping))
		for _, pm := range ep.portMapping {
			pmc = append(pmc, pm.GetCopy())
		}
		data[netlabel.PortMap] = pmc
	}

	return data, nil
}

func endpointRequest(method, path, request string) (*hcsshim.HNSEndpoint, error) {
	if windowsBuild == 14393 {
		endpointMu.Lock()
	}
	hnsresponse, err := hcsshim.HNSEndpointRequest(method, path, request)
	if windowsBuild == 14393 {
		endpointMu.Unlock()
	}
	return hnsresponse, err
}
