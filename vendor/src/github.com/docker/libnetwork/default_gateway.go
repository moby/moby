package libnetwork

import (
	"fmt"

	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
)

const (
	libnGWNetwork = "docker_gwbridge"
	gwEPlen       = 12
)

/*
   libnetwork creates a bridge network "docker_gw_bridge" for provding
   default gateway for the containers if none of the container's endpoints
   have GW set by the driver. ICC is set to false for the GW_bridge network.

   If a driver can't provide external connectivity it can choose to not set
   the GW IP for the endpoint.

   endpoint on the GW_bridge network is managed dynamically by libnetwork.
   ie:
   - its created when an endpoint without GW joins the container
   - its deleted when an endpoint with GW joins the container
*/

func (sb *sandbox) setupDefaultGW(srcEp *endpoint) error {
	var createOptions []EndpointOption
	c := srcEp.getNetwork().getController()

	// check if the conitainer already has a GW endpoint
	if ep := sb.getEndpointInGWNetwork(); ep != nil {
		return nil
	}

	n, err := c.NetworkByName(libnGWNetwork)
	if err != nil {
		if _, ok := err.(types.NotFoundError); !ok {
			return err
		}
		n, err = c.createGWNetwork()
		if err != nil {
			return err
		}
	}

	if opt, ok := srcEp.generic[netlabel.PortMap]; ok {
		if pb, ok := opt.([]types.PortBinding); ok {
			createOptions = append(createOptions, CreateOptionPortMapping(pb))
		}
	}

	if opt, ok := srcEp.generic[netlabel.ExposedPorts]; ok {
		if exp, ok := opt.([]types.TransportPort); ok {
			createOptions = append(createOptions, CreateOptionExposedPorts(exp))
		}
	}

	createOptions = append(createOptions, CreateOptionAnonymous())

	eplen := gwEPlen
	if len(sb.containerID) < gwEPlen {
		eplen = len(sb.containerID)
	}

	newEp, err := n.CreateEndpoint("gateway_"+sb.containerID[0:eplen], createOptions...)
	if err != nil {
		return fmt.Errorf("container %s: endpoint create on GW Network failed: %v", sb.containerID, err)
	}
	epLocal := newEp.(*endpoint)

	if err := epLocal.sbJoin(sb); err != nil {
		return fmt.Errorf("container %s: endpoint join on GW Network failed: %v", sb.containerID, err)
	}
	return nil
}

func (sb *sandbox) clearDefaultGW() error {
	var ep *endpoint

	if ep = sb.getEndpointInGWNetwork(); ep == nil {
		return nil
	}

	if err := ep.sbLeave(sb); err != nil {
		return fmt.Errorf("container %s: endpoint leaving GW Network failed: %v", sb.containerID, err)
	}
	if err := ep.Delete(); err != nil {
		return fmt.Errorf("container %s: deleting endpoint on GW Network failed: %v", sb.containerID, err)
	}
	return nil
}

func (sb *sandbox) needDefaultGW() bool {
	var needGW bool

	for _, ep := range sb.getConnectedEndpoints() {
		if ep.endpointInGWNetwork() {
			continue
		}
		if ep.getNetwork().Type() == "null" || ep.getNetwork().Type() == "host" {
			continue
		}
		// TODO v6 needs to be handled.
		if len(ep.Gateway()) > 0 {
			return false
		}
		needGW = true
	}
	return needGW
}

func (sb *sandbox) getEndpointInGWNetwork() *endpoint {
	for _, ep := range sb.getConnectedEndpoints() {
		if ep.getNetwork().name == libnGWNetwork {
			return ep
		}
	}
	return nil
}

func (ep *endpoint) endpointInGWNetwork() bool {
	if ep.getNetwork().name == libnGWNetwork {
		return true
	}
	return false
}

func (sb *sandbox) getEPwithoutGateway() *endpoint {
	for _, ep := range sb.getConnectedEndpoints() {
		if ep.getNetwork().Type() == "null" || ep.getNetwork().Type() == "host" {
			continue
		}
		if len(ep.Gateway()) == 0 {
			return ep
		}
	}
	return nil
}
