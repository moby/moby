package libnetwork

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

// Populate all loadbalancers on the network that the passed endpoint
// belongs to, into this sandbox.
func (sb *Sandbox) populateLoadBalancers(ep *Endpoint) {
	// This is an interface less endpoint. Nothing to do.
	if ep.Iface() == nil {
		return
	}

	n := ep.getNetwork()
	eIP := ep.Iface().Address()

	if n.ingress {
		if err := sb.addRedirectRules(eIP, ep.ingressPorts); err != nil {
			log.G(context.TODO()).Errorf("Failed to add redirect rules for ep %s (%.7s): %v", ep.Name(), ep.ID(), err)
		}
	}
}

func (n *Network) findLBEndpointSandbox() (*Endpoint, *Sandbox, error) {
	// TODO: get endpoint from store?  See EndpointInfo()
	var ep *Endpoint
	// Find this node's LB sandbox endpoint:  there should be exactly one
	for _, e := range n.Endpoints() {
		epi := e.Info()
		if epi != nil && epi.LoadBalancer() {
			ep = e
			break
		}
	}
	if ep == nil {
		return nil, nil, fmt.Errorf("Unable to find load balancing endpoint for network %s", n.ID())
	}
	// Get the load balancer sandbox itself as well
	sb, ok := ep.getSandbox()
	if !ok {
		return nil, nil, fmt.Errorf("Unable to get sandbox for %s(%s) in for %s", ep.Name(), ep.ID(), n.ID())
	}
	sep := sb.GetEndpoint(ep.ID())
	if sep == nil {
		return nil, nil, fmt.Errorf("Load balancing endpoint %s(%s) removed from %s", ep.Name(), ep.ID(), n.ID())
	}
	return sep, sb, nil
}

// Searches the OS sandbox for the name of the endpoint interface
// within the sandbox.   This is required for adding/removing IP
// aliases to the interface.
func findIfaceDstName(sb *Sandbox, ep *Endpoint) string {
	srcName := ep.Iface().SrcName()
	for _, i := range sb.osSbox.Interfaces() {
		if i.SrcName() == srcName {
			return i.DstName()
		}
	}
	return ""
}

// Add loadbalancer backend to the loadbalancer sandbox for the network.
// If needed add the service as well.
func (n *Network) addLBBackend(ip net.IP, lb *loadBalancer) {
	if len(lb.vip) == 0 {
		return
	}
	newService := n.addLBBackendIPTables(ip, lb)

	if newService && n.ingress {
		_, sb, err := n.findLBEndpointSandbox()
		if err != nil {
			log.G(context.TODO()).Errorf("Failed to find load balancer endpoint sandbox: %v", err)
			return
		}
		gwEP, _ := sb.getGatewayEndpoint()
		if gwEP == nil {
			log.G(context.TODO()).Errorf("Failed to add ingress ports: no gateway endpoint for sandbox %.7s", sb.ID())
			return
		}
		if err := addIngressPorts(gwEP, lb.service.ingressPorts); err != nil {
			log.G(context.TODO()).Errorf("Failed to add ingress: %v", err)
		}
	}
}

// Remove loadbalancer backend the load balancing endpoint for this
// network. If 'rmService' is true, then remove the service entry as well.
// If 'fullRemove' is true then completely remove the entry, otherwise
// just deweight it for now.
func (n *Network) rmLBBackend(ip net.IP, lb *loadBalancer, rmService bool, fullRemove bool) {
	if len(lb.vip) == 0 {
		return
	}
	n.rmLBBackendIPTables(ip, lb, rmService, fullRemove)

	if rmService && n.ingress {
		_, sb, err := n.findLBEndpointSandbox()
		if err != nil {
			log.G(context.TODO()).Errorf("Failed to find load balancer endpoint sandbox: %v", err)
			return
		}
		if gwEP, _ := sb.getGatewayEndpoint(); gwEP == nil {
			log.G(context.TODO()).Errorf("Failed to remove ingress ports: no gateway endpoint for sandbox %.7s", sb.ID())
		} else if err := removeIngressPorts(gwEP, lb.service.ingressPorts); err != nil {
			log.G(context.TODO()).Errorf("Failed to remove ingress: %v", err)
		}
	}
}

var (
	ingressMu     sync.Mutex // lock for operations on ingress
	portConfigMu  sync.Mutex
	portConfigTbl = make(map[PortConfig]int)
)

func filterPortConfigs(ingressPorts []*PortConfig, isDelete bool) []*PortConfig {
	portConfigMu.Lock()
	iPorts := make([]*PortConfig, 0, len(ingressPorts))
	for _, pc := range ingressPorts {
		if isDelete {
			if cnt, ok := portConfigTbl[*pc]; ok {
				// This is the last reference to this
				// port config. Delete the port config
				// and add it to filtered list to be
				// plumbed.
				if cnt == 1 {
					delete(portConfigTbl, *pc)
					iPorts = append(iPorts, pc)
					continue
				}

				portConfigTbl[*pc] = cnt - 1
			}

			continue
		}

		if cnt, ok := portConfigTbl[*pc]; ok {
			portConfigTbl[*pc] = cnt + 1
			continue
		}

		// We are adding it for the first time. Add it to the
		// filter list to be plumbed.
		portConfigTbl[*pc] = 1
		iPorts = append(iPorts, pc)
	}
	portConfigMu.Unlock()

	return iPorts
}

// ingressPortsToBindings converts swarm ingress port configs into the port
// bindings to publish on the load-balancer sandbox's docker_gwbridge gateway
// endpoint. Each published port is mapped straight through to the same port on
// the gateway endpoint's address; the in-sandbox redirect rules then translate
// it to the service's target port.
func ingressPortsToBindings(ingressPorts []*PortConfig) []types.PortBinding {
	pbs := make([]types.PortBinding, 0, len(ingressPorts))
	for _, p := range ingressPorts {
		pbs = append(pbs, types.PortBinding{
			Proto:       types.ParseProtocol(strings.ToLower(p.Protocol.String())),
			Port:        uint16(p.PublishedPort),
			HostPort:    uint16(p.PublishedPort),
			HostPortEnd: uint16(p.PublishedPort),
		})
	}
	return pbs
}

// removeIngressPorts unpublishes the ingress ports whose last reference is being
// removed from the load-balancer sandbox's gateway endpoint.
func removeIngressPorts(gwEP *Endpoint, ingressPorts []*PortConfig) error {
	ingressMu.Lock()
	defer ingressMu.Unlock()

	filteredPorts := filterPortConfigs(ingressPorts, true)
	if len(filteredPorts) == 0 {
		return nil
	}
	if err := gwEP.DelPorts(context.TODO(), ingressPortsToBindings(filteredPorts)); err != nil {
		filterPortConfigs(ingressPorts, false)
		return fmt.Errorf("failed to remove ingress ports: %v", err)
	}
	return nil
}

// addIngressPorts publishes the ingress ports being referenced for the first
// time on the load-balancer sandbox's gateway endpoint, reusing the bridge
// driver's port-publishing machinery (DNAT + forwarding rules + host-port
// reservation).
func addIngressPorts(gwEP *Endpoint, ingressPorts []*PortConfig) error {
	ingressMu.Lock()
	defer ingressMu.Unlock()

	filteredPorts := filterPortConfigs(ingressPorts, false)
	if len(filteredPorts) == 0 {
		return nil
	}
	// Ingress ports are re-derived from the cluster and re-added on
	// restart, so they must not be saved to the bridge driver's store (else
	// they'd be restored and then published a second time).
	if err := gwEP.AddEphemeralPorts(context.TODO(), ingressPortsToBindings(filteredPorts)); err != nil {
		// filterPortConfigs(_, false) above bumped the refcount of every port in
		// ingressPorts, not just the newly-referenced filteredPorts, so roll back
		// the whole set to keep the counts balanced.
		filterPortConfigs(ingressPorts, true)
		return fmt.Errorf("failed to program ingress ports: %v", err)
	}
	return nil
}

func (sb *Sandbox) addRedirectRules(eIP *net.IPNet, ingressPorts []*PortConfig) error {
	return sb.addRedirectRulesIPTables(eIP, ingressPorts)
}
