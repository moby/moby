package libnetwork

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
)

const (
	gwEPlen = 12
)

var procGwNetwork = make(chan (bool), 1)

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

	newEp, err := n.CreateEndpoint(context.TODO(), gwName, createOptions...)
	if err != nil {
		return fmt.Errorf("container %s: endpoint create on GW Network failed: %v", sb.containerID, err)
	}

	defer func() {
		if err != nil {
			if err2 := newEp.Delete(context.WithoutCancel(context.TODO()), true); err2 != nil {
				log.G(context.TODO()).Warnf("Failed to remove gw endpoint for container %s after failing to join the gateway network: %v",
					sb.containerID, err2)
			}
		}
	}()

	if err = newEp.sbJoin(context.TODO(), sb); err != nil {
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
	if err := ep.sbLeave(context.TODO(), sb, false); err != nil {
		return fmt.Errorf("container %s: endpoint leaving GW Network failed: %v", sb.containerID, err)
	}
	if err := ep.Delete(context.TODO(), false); err != nil {
		return fmt.Errorf("container %s: deleting endpoint on GW Network failed: %v", sb.containerID, err)
	}
	return nil
}

// Evaluate whether the sandbox requires a default gateway based
// on the endpoints to which it is connected. It does not account
// for the default gateway network endpoint.

func (sb *Sandbox) needDefaultGW() bool {
	var needGW bool

	for _, ep := range sb.Endpoints() {
		if ep.endpointInGWNetwork() {
			continue
		}
		if ep.getNetwork().Type() == "null" || ep.getNetwork().Type() == "host" {
			continue
		}
		if ep.getNetwork().Internal() {
			continue
		}
		// During stale sandbox cleanup, joinInfo may be nil
		if ep.joinInfo != nil && ep.joinInfo.disableGatewayService {
			continue
		}
		if len(ep.Gateway()) > 0 {
			return false
		}
		if len(ep.GatewayIPv6()) > 0 {
			return false
		}
		for _, r := range ep.StaticRoutes() {
			if r.Destination != nil && r.Destination.String() == "0.0.0.0/0" {
				return false
			}
		}
		needGW = true
	}

	return needGW
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

// Looks for the default gw network and creates it if not there.
// Parallel executions are serialized.
func (c *Controller) defaultGwNetwork() (*Network, error) {
	procGwNetwork <- true
	defer func() { <-procGwNetwork }()

	n, err := c.NetworkByName(libnGWNetwork)
	if errdefs.IsNotFound(err) {
		n, err = c.createGWNetwork()
	}
	return n, err
}

// getGatewayEndpoint returns the endpoints providing external connectivity to
// the sandbox. If the gateway is dual-stack, ep4 and ep6 will point at the same
// endpoint. If there is no IPv4/IPv6 connectivity, nil pointers will be returned.
func (sb *Sandbox) getGatewayEndpoint() (ep4, ep6 *Endpoint) {
	return selectGatewayEndpoint(sb.Endpoints())
}

// selectGatewayEndpoint is like getGatewayEndpoint, but selects only from
// endpoints.
func selectGatewayEndpoint(endpoints []*Endpoint) (ep4, ep6 *Endpoint) {
	for _, ep := range endpoints {
		if ep.getNetwork().Type() == "null" || ep.getNetwork().Type() == "host" {
			continue
		}
		gw4, gw6 := ep.hasGatewayOrDefaultRoute()
		if gw4 && gw6 {
			// The first dual-stack endpoint is the gateway, no need to search further.
			//
			// FIXME(robmry) - this means a dual-stack gateway is preferred over single-stack
			// gateways with higher gateway-priorities. A dual-stack network should probably
			// be preferred over two single-stack networks, if they all have equal priorities.
			// It'd probably also be better to use a dual-stack endpoint as the gateway for
			// a single address family, if there's a higher-priority single-stack gateway for
			// the other address family. (But, priority is currently a Sandbox property, not
			// an Endpoint property. So, this function doesn't have access to priorities.)
			return ep, ep
		}
		if gw4 && ep4 == nil {
			// Found the best IPv4-only gateway, keep searching for an IPv6 or dual-stack gateway.
			ep4 = ep
		}
		if gw6 && ep6 == nil {
			// Found the best IPv6-only gateway, keep searching for an IPv4 or dual-stack gateway.
			ep6 = ep
		}
	}
	return ep4, ep6
}
