package libnetwork

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/ishidawataru/sctp"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
)

type osLoadBalancer struct {
	nftClearNAT, nftClearDSR nftables.Modifier
	nftBackendsProgrammed    int
}

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
	var newService bool
	if nftables.Enabled() {
		newService = n.syncLBBackendsNftables(context.TODO(), lb, false)
	} else {
		newService = n.addLBBackendIPTables(ip, lb)
	}

	if newService && n.ingress {
		_, sb, err := n.findLBEndpointSandbox()
		if err != nil {
			log.G(context.TODO()).Errorf("Failed to find load balancer endpoint sandbox: %v", err)
			return
		}
		var gwIP net.IP
		if gwEP, _ := sb.getGatewayEndpoint(); gwEP != nil {
			gwIP = gwEP.Iface().Address().IP
		}
		if err := addIngressPorts(gwIP, lb.service.ingressPorts); err != nil {
			log.G(context.TODO()).Errorf("Failed to add ingress: %v", err)
			return
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
	if nftables.Enabled() {
		n.syncLBBackendsNftables(context.TODO(), lb, rmService)
	} else {
		n.rmLBBackendIPTables(ip, lb, rmService, fullRemove)
	}

	if rmService && n.ingress {
		_, sb, err := n.findLBEndpointSandbox()
		if err != nil {
			log.G(context.TODO()).Errorf("Failed to find load balancer endpoint sandbox: %v", err)
			return
		}
		var gwIP net.IP
		if gwEP, _ := sb.getGatewayEndpoint(); gwEP != nil {
			gwIP = gwEP.Iface().Address().IP
		}
		if err := removeIngressPorts(gwIP, lb.service.ingressPorts); err != nil {
			log.G(context.TODO()).Errorf("Failed to remove ingress: %v", err)
			return
		}
	}
}

var (
	ingressMu       sync.Mutex // lock for operations on ingress
	ingressProxyTbl = make(map[string]io.Closer)
	portConfigMu    sync.Mutex
	portConfigTbl   = make(map[PortConfig]int)
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

func removeIngressPorts(gwIP net.IP, ingressPorts []*PortConfig) error {
	// TODO IPv6 support

	ingressMu.Lock()
	defer ingressMu.Unlock()

	// Filter the ingress ports until port rules start to be added/deleted
	filteredPorts := filterPortConfigs(ingressPorts, true)

	var err error
	if nftables.Enabled() {
		err = deleteIngressPortsNftables(context.TODO(), filteredPorts)
	} else {
		err = deleteIngressPortsRulesIPTables(gwIP, filteredPorts)
	}
	if err != nil {
		filterPortConfigs(ingressPorts, false)
		return fmt.Errorf("failed to program ingress ports: %v", err)
	}

	closeIngressPortsProxy(filteredPorts)

	return nil
}

func addIngressPorts(gwIP net.IP, ingressPorts []*PortConfig) error {
	ingressMu.Lock()
	defer ingressMu.Unlock()

	// Filter the ingress ports until port rules start to be added/deleted
	filteredPorts := filterPortConfigs(ingressPorts, false)

	var err error
	if nftables.Enabled() {
		err = addIngressPortsNftables(context.TODO(), gwIP, filteredPorts)
	} else {
		err = programIngressPortsRulesIPTables(gwIP, filteredPorts)
	}
	if err != nil {
		filterPortConfigs(filteredPorts, true)
		return fmt.Errorf("failed to program ingress ports: %v", err)
	}

	plumbIngressPortsProxy(filteredPorts)

	return nil
}

func restoreIngressPorts(gwIP net.IP, ingressPorts []*PortConfig) error {
	if nftables.Enabled() {
		// No-op in nftables world as firewalld doesn't touch our tables
		// and we don't touch theirs.
		return nil
	}

	ingressMu.Lock()
	defer ingressMu.Unlock()

	if err := programIngressPortsRulesIPTables(gwIP, ingressPorts); err != nil {
		return fmt.Errorf("failed to program ingress ports: %v", err)
	}

	return nil
}

func findOIFName(ip net.IP) (string, error) {
	nlh := ns.NlHandle()

	routes, err := nlh.RouteGet(ip)
	if err != nil {
		return "", err
	}

	if len(routes) == 0 {
		return "", fmt.Errorf("no route to %s", ip)
	}

	// Pick the first route(typically there is only one route). We
	// don't support multipath.
	link, err := nlh.LinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return "", err
	}

	return link.Attrs().Name, nil
}

func closeIngressPortsProxy(ingressPorts []*PortConfig) {
	for _, iPort := range ingressPorts {
		portSpec := fmt.Sprintf("%d/%s", iPort.PublishedPort, strings.ToLower(iPort.Protocol.String()))
		listener, ok := ingressProxyTbl[portSpec]
		if !ok {
			continue
		}

		if listener != nil {
			listener.Close()
		}
		delete(ingressProxyTbl, portSpec)
	}
}

func plumbIngressPortsProxy(ingressPorts []*PortConfig) {
	var (
		err error
		l   io.Closer
	)

	for _, iPort := range ingressPorts {
		portSpec := fmt.Sprintf("%d/%s", iPort.PublishedPort, strings.ToLower(iPort.Protocol.String()))
		listener, ok := ingressProxyTbl[portSpec]
		if ok && listener != nil {
			continue // already listening on this port
		}

		switch iPort.Protocol {
		case ProtocolTCP:
			l, err = net.ListenTCP("tcp", &net.TCPAddr{Port: int(iPort.PublishedPort)})
		case ProtocolUDP:
			l, err = net.ListenUDP("udp", &net.UDPAddr{Port: int(iPort.PublishedPort)})
		case ProtocolSCTP:
			l, err = sctp.ListenSCTP("sctp", &sctp.SCTPAddr{Port: int(iPort.PublishedPort)})
		default:
			err = fmt.Errorf("unknown protocol %v", iPort.Protocol)
		}

		if err != nil {
			log.G(context.TODO()).Warnf("failed to create proxy for port %s: %v", iPort, err)
		}

		ingressProxyTbl[portSpec] = l
	}
}

func (sb *Sandbox) addRedirectRules(eIP *net.IPNet, ingressPorts []*PortConfig) error {
	if nftables.Enabled() {
		return sb.addRedirectRulesNftables(context.TODO(), eIP, ingressPorts)
	} else {
		return sb.addRedirectRulesIPTables(eIP, ingressPorts)
	}
}
