package libnetwork

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
)

const (
	gwEPlen = 12
)

/*
   libnetwork creates a bridge network "docker_gw_bridge" for providing
   default gateway for the containers if none of the container's endpoints
   have GW set by the driver. ICC is set to false for the GW_bridge network.

   If a driver can't provide external connectivity it can choose to not set
   the GW IP for the endpoint.

   endpoint on the GW_bridge network is managed dynamically by libnetwork.
   ie:
   - its created when an endpoint without GW joins the container
   - its deleted when an endpoint with GW joins the container
*/

func (sb *Sandbox) setupDefaultGW() error {
	// check if the container already has a GW endpoint
	if ep := sb.getEndpointInGWNetwork(); ep != nil {
		return nil
	}

	c := sb.controller

	// Look for default gw network. In case of error (includes not found),
	// retry and create it if needed in a serialized execution.
	n, err := c.NetworkByName(libnGWNetwork)
	if err != nil {
		if n, err = c.defaultGwNetwork(); err != nil {
			return err
		}
	}

	createOptions := []EndpointOption{}

	var gwName string
	if len(sb.containerID) <= gwEPlen {
		gwName = "gateway_" + sb.containerID
	} else {
		gwName = "gateway_" + sb.id[:gwEPlen]
	}

	sbLabels := sb.Labels()

	if sbLabels[netlabel.PortMap] != nil {
		createOptions = append(createOptions, CreateOptionPortMapping(sbLabels[netlabel.PortMap].([]types.PortBinding)))
	}

	if sbLabels[netlabel.ExposedPorts] != nil {
		createOptions = append(createOptions, CreateOptionExposedPorts(sbLabels[netlabel.ExposedPorts].([]types.TransportPort)))
	}

	epOption := getPlatformOption()
	if epOption != nil {
		createOptions = append(createOptions, epOption)
	}

	newEp, err := n.CreateEndpoint(gwName, createOptions...)
	if err != nil {
		return fmt.Errorf("container %s: endpoint create on GW Network failed: %v", sb.containerID, err)
	}

	defer func() {
		if err != nil {
			if err2 := newEp.Delete(true); err2 != nil {
				log.G(context.TODO()).Warnf("Failed to remove gw endpoint for container %s after failing to join the gateway network: %v",
					sb.containerID, err2)
			}
		}
	}()

	if err = newEp.sbJoin(sb); err != nil {
		return fmt.Errorf("container %s: endpoint join on GW Network failed: %v", sb.containerID, err)
	}

	return nil
}

// If present, detach and remove the endpoint connecting the sandbox to the default gw network.
func (sb *Sandbox) clearDefaultGW() error {
	var ep *Endpoint

	if ep = sb.getEndpointInGWNetwork(); ep == nil {
		return nil
	}
	if err := ep.sbLeave(sb, false); err != nil {
		return fmt.Errorf("container %s: endpoint leaving GW Network failed: %v", sb.containerID, err)
	}
	if err := ep.Delete(false); err != nil {
		return fmt.Errorf("container %s: deleting endpoint on GW Network failed: %v", sb.containerID, err)
	}
	return nil
}

// needDefaultGW evaluates whether the sandbox needs to be connected to the
// 'docker_gwbridge' network based on the endpoints to which it is connected
// to (ie. at least one endpoint should require it, and no other endpoint
// should provide a gateway).
func (sb *Sandbox) needDefaultGW() bool {
	for _, ep := range sb.Endpoints() {
		if ep.endpointInGWNetwork() {
			// There's already an endpoint attached to docker_gwbridge. This sandbox doesn't need to be attached to it
			// once again.
			return false
		}
		if ep.isGateway() {
			// This endpoint already provides a gateway. This sandbox doesn't need to be attached to docker_gwbridge.
			return false
		}
		// The 'remote' netdriver shim doesn't store anything across [driverapi.Driver] operations. So, by the time its
		// Join method gets called, it already lost the information of whether the network is internal. Hence here we
		// need to check whether the network is internal. Also, during stale sandbox cleanup, joinInfo may be nil.
		if !ep.getNetwork().Internal() && ep.joinInfo != nil && ep.joinInfo.requireDefaultGateway {
			return true
		}
	}

	return false
}

func (sb *Sandbox) getEndpointInGWNetwork() *Endpoint {
	for _, ep := range sb.Endpoints() {
		if ep.getNetwork().name == libnGWNetwork && strings.HasPrefix(ep.Name(), "gateway_") {
			return ep
		}
	}
	return nil
}

func (ep *Endpoint) endpointInGWNetwork() bool {
	if ep.getNetwork().name == libnGWNetwork && strings.HasPrefix(ep.Name(), "gateway_") {
		return true
	}
	return false
}

// defaultGwNetwork looks for the 'docker_gwbridge' network and creates it if
// it doesn't exist. It's safe for concurrent use.
func (c *Controller) defaultGwNetwork() (*Network, error) {
	n, err := c.NetworkByName(libnGWNetwork)
	if _, ok := err.(types.NotFoundError); ok {
		n, err = c.createGWNetwork()
		// If two concurrent calls to this method are made, they will both try
		// to create the 'docker_gwbridge' network concurrently, but ultimately
		// the Controller will serialize 'NewNetwork' calls. The first call to
		// win the race will create the network, and other racing calls will
		// find out it already exists and return a 'NetworkNameError'. Instead
		// of barfing out, just try to retrieve it once more.
		if errors.Is(err, NetworkNameError(libnGWNetwork)) {
			return c.NetworkByName(libnGWNetwork)
		}
	}
	return n, err
}

// Returns the endpoint which is providing external connectivity to the sandbox
func (sb *Sandbox) getGatewayEndpoint() *Endpoint {
	for _, ep := range sb.Endpoints() {
		if ep.getNetwork().Type() == "null" || ep.getNetwork().Type() == "host" {
			continue
		}
		if ep.isGateway() {
			return ep
		}
	}
	return nil
}

// isGateway determines whether this endpoint provides a gateway.
func (ep *Endpoint) isGateway() bool {
	if len(ep.Gateway()) > 0 {
		return true
	}

	for _, r := range ep.StaticRoutes() {
		if r.Destination != nil && r.Destination.String() == "0.0.0.0/0" {
			return true
		}
	}

	return false
}
