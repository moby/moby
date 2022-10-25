package libnetwork

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/pkg/reexec"
	"github.com/gogo/protobuf/proto"
	"github.com/ishidawataru/sctp"
	"github.com/moby/ipvs"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("fwmarker", fwMarker)
	reexec.Register("redirector", redirector)
}

// Populate all loadbalancers on the network that the passed endpoint
// belongs to, into this sandbox.
func (sb *sandbox) populateLoadBalancers(ep *endpoint) {
	// This is an interface less endpoint. Nothing to do.
	if ep.Iface() == nil {
		return
	}

	n := ep.getNetwork()
	eIP := ep.Iface().Address()

	if n.ingress {
		if err := addRedirectRules(sb.Key(), eIP, ep.ingressPorts); err != nil {
			logrus.Errorf("Failed to add redirect rules for ep %s (%.7s): %v", ep.Name(), ep.ID(), err)
		}
	}
}

func (n *network) findLBEndpointSandbox() (*endpoint, *sandbox, error) {
	// TODO: get endpoint from store?  See EndpointInfo()
	var ep *endpoint
	// Find this node's LB sandbox endpoint:  there should be exactly one
	for _, e := range n.Endpoints() {
		epi := e.Info()
		if epi != nil && epi.LoadBalancer() {
			ep = e.(*endpoint)
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
	sep := sb.getEndpoint(ep.ID())
	if sep == nil {
		return nil, nil, fmt.Errorf("Load balancing endpoint %s(%s) removed from %s", ep.Name(), ep.ID(), n.ID())
	}
	return sep, sb, nil
}

// Searches the OS sandbox for the name of the endpoint interface
// within the sandbox.   This is required for adding/removing IP
// aliases to the interface.
func findIfaceDstName(sb *sandbox, ep *endpoint) string {
	srcName := ep.Iface().SrcName()
	for _, i := range sb.osSbox.Info().Interfaces() {
		if i.SrcName() == srcName {
			return i.DstName()
		}
	}
	return ""
}

// Add loadbalancer backend to the loadbalncer sandbox for the network.
// If needed add the service as well.
func (n *network) addLBBackend(ip net.IP, lb *loadBalancer) {
	if len(lb.vip) == 0 {
		return
	}
	ep, sb, err := n.findLBEndpointSandbox()
	if err != nil {
		logrus.Errorf("addLBBackend %s/%s: %v", n.ID(), n.Name(), err)
		return
	}
	if sb.osSbox == nil {
		return
	}

	eIP := ep.Iface().Address()

	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create an ipvs handle for sbox %.7s (%.7s,%s) for lb addition: %v", sb.ID(), sb.ContainerID(), sb.Key(), err)
		return
	}
	defer i.Close()

	s := &ipvs.Service{
		AddressFamily: nl.FAMILY_V4,
		FWMark:        lb.fwMark,
		SchedName:     ipvs.RoundRobin,
	}

	if !i.IsServicePresent(s) {
		// Add IP alias for the VIP to the endpoint
		ifName := findIfaceDstName(sb, ep)
		if ifName == "" {
			logrus.Errorf("Failed find interface name for endpoint %s(%s) to create LB alias", ep.ID(), ep.Name())
			return
		}
		err := sb.osSbox.AddAliasIP(ifName, &net.IPNet{IP: lb.vip, Mask: net.CIDRMask(32, 32)})
		if err != nil {
			logrus.Errorf("Failed add IP alias %s to network %s LB endpoint interface %s: %v", lb.vip, n.ID(), ifName, err)
			return
		}

		if sb.ingress {
			var gwIP net.IP
			if ep := sb.getGatewayEndpoint(); ep != nil {
				gwIP = ep.Iface().Address().IP
			}
			if err := programIngress(gwIP, lb.service.ingressPorts, false); err != nil {
				logrus.Errorf("Failed to add ingress: %v", err)
				return
			}
		}

		logrus.Debugf("Creating service for vip %s fwMark %d ingressPorts %#v in sbox %.7s (%.7s)", lb.vip, lb.fwMark, lb.service.ingressPorts, sb.ID(), sb.ContainerID())
		if err := invokeFWMarker(sb.Key(), lb.vip, lb.fwMark, lb.service.ingressPorts, eIP, false, n.loadBalancerMode); err != nil {
			logrus.Errorf("Failed to add firewall mark rule in sbox %.7s (%.7s): %v", sb.ID(), sb.ContainerID(), err)
			return
		}

		if err := i.NewService(s); err != nil && err != syscall.EEXIST {
			logrus.Errorf("Failed to create a new service for vip %s fwmark %d in sbox %.7s (%.7s): %v", lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
			return
		}
	}

	d := &ipvs.Destination{
		AddressFamily: nl.FAMILY_V4,
		Address:       ip,
		Weight:        1,
	}
	if n.loadBalancerMode == loadBalancerModeDSR {
		d.ConnectionFlags = ipvs.ConnFwdDirectRoute
	}

	// Remove the sched name before using the service to add
	// destination.
	s.SchedName = ""
	if err := i.NewDestination(s, d); err != nil && err != syscall.EEXIST {
		logrus.Errorf("Failed to create real server %s for vip %s fwmark %d in sbox %.7s (%.7s): %v", ip, lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
	}

	// Ensure that kernel tweaks are applied in case this is the first time
	// we've initialized ip_vs
	sb.osSbox.ApplyOSTweaks(sb.oslTypes)
}

// Remove loadbalancer backend the load balancing endpoint for this
// network. If 'rmService' is true, then remove the service entry as well.
// If 'fullRemove' is true then completely remove the entry, otherwise
// just deweight it for now.
func (n *network) rmLBBackend(ip net.IP, lb *loadBalancer, rmService bool, fullRemove bool) {
	if len(lb.vip) == 0 {
		return
	}
	ep, sb, err := n.findLBEndpointSandbox()
	if err != nil {
		logrus.Debugf("rmLBBackend for %s/%s: %v -- probably transient state", n.ID(), n.Name(), err)
		return
	}
	if sb.osSbox == nil {
		return
	}

	eIP := ep.Iface().Address()

	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create an ipvs handle for sbox %.7s (%.7s,%s) for lb removal: %v", sb.ID(), sb.ContainerID(), sb.Key(), err)
		return
	}
	defer i.Close()

	s := &ipvs.Service{
		AddressFamily: nl.FAMILY_V4,
		FWMark:        lb.fwMark,
	}

	d := &ipvs.Destination{
		AddressFamily: nl.FAMILY_V4,
		Address:       ip,
		Weight:        1,
	}
	if n.loadBalancerMode == loadBalancerModeDSR {
		d.ConnectionFlags = ipvs.ConnFwdDirectRoute
	}

	if fullRemove {
		if err := i.DelDestination(s, d); err != nil && err != syscall.ENOENT {
			logrus.Errorf("Failed to delete real server %s for vip %s fwmark %d in sbox %.7s (%.7s): %v", ip, lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
		}
	} else {
		d.Weight = 0
		if err := i.UpdateDestination(s, d); err != nil && err != syscall.ENOENT {
			logrus.Errorf("Failed to set LB weight of real server %s to 0 for vip %s fwmark %d in sbox %.7s (%.7s): %v", ip, lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
		}
	}

	if rmService {
		s.SchedName = ipvs.RoundRobin
		if err := i.DelService(s); err != nil && err != syscall.ENOENT {
			logrus.Errorf("Failed to delete service for vip %s fwmark %d in sbox %.7s (%.7s): %v", lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
		}

		if sb.ingress {
			var gwIP net.IP
			if ep := sb.getGatewayEndpoint(); ep != nil {
				gwIP = ep.Iface().Address().IP
			}
			if err := programIngress(gwIP, lb.service.ingressPorts, true); err != nil {
				logrus.Errorf("Failed to delete ingress: %v", err)
			}
		}

		if err := invokeFWMarker(sb.Key(), lb.vip, lb.fwMark, lb.service.ingressPorts, eIP, true, n.loadBalancerMode); err != nil {
			logrus.Errorf("Failed to delete firewall mark rule in sbox %.7s (%.7s): %v", sb.ID(), sb.ContainerID(), err)
		}

		// Remove IP alias from the VIP to the endpoint
		ifName := findIfaceDstName(sb, ep)
		if ifName == "" {
			logrus.Errorf("Failed find interface name for endpoint %s(%s) to create LB alias", ep.ID(), ep.Name())
			return
		}
		err := sb.osSbox.RemoveAliasIP(ifName, &net.IPNet{IP: lb.vip, Mask: net.CIDRMask(32, 32)})
		if err != nil {
			logrus.Errorf("Failed add IP alias %s to network %s LB endpoint interface %s: %v", lb.vip, n.ID(), ifName, err)
		}
	}
}

const ingressChain = "DOCKER-INGRESS"

var (
	ingressOnce     sync.Once
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

func programIngress(gwIP net.IP, ingressPorts []*PortConfig, isDelete bool) error {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)

	addDelOpt := "-I"
	rollbackAddDelOpt := "-D"
	if isDelete {
		addDelOpt = "-D"
		rollbackAddDelOpt = "-I"
	}

	ingressMu.Lock()
	defer ingressMu.Unlock()

	chainExists := iptable.ExistChain(ingressChain, iptables.Nat)
	filterChainExists := iptable.ExistChain(ingressChain, iptables.Filter)

	ingressOnce.Do(func() {
		// Flush nat table and filter table ingress chain rules during init if it
		// exists. It might contain stale rules from previous life.
		if chainExists {
			if err := iptable.RawCombinedOutput("-t", "nat", "-F", ingressChain); err != nil {
				logrus.Errorf("Could not flush nat table ingress chain rules during init: %v", err)
			}
		}
		if filterChainExists {
			if err := iptable.RawCombinedOutput("-F", ingressChain); err != nil {
				logrus.Errorf("Could not flush filter table ingress chain rules during init: %v", err)
			}
		}
	})

	if !isDelete {
		if !chainExists {
			if err := iptable.RawCombinedOutput("-t", "nat", "-N", ingressChain); err != nil {
				return fmt.Errorf("failed to create ingress chain: %v", err)
			}
		}
		if !filterChainExists {
			if err := iptable.RawCombinedOutput("-N", ingressChain); err != nil {
				return fmt.Errorf("failed to create filter table ingress chain: %v", err)
			}
		}

		if !iptable.Exists(iptables.Nat, ingressChain, "-j", "RETURN") {
			if err := iptable.RawCombinedOutput("-t", "nat", "-A", ingressChain, "-j", "RETURN"); err != nil {
				return fmt.Errorf("failed to add return rule in nat table ingress chain: %v", err)
			}
		}

		if !iptable.Exists(iptables.Filter, ingressChain, "-j", "RETURN") {
			if err := iptable.RawCombinedOutput("-A", ingressChain, "-j", "RETURN"); err != nil {
				return fmt.Errorf("failed to add return rule to filter table ingress chain: %v", err)
			}
		}

		for _, chain := range []string{"OUTPUT", "PREROUTING"} {
			if !iptable.Exists(iptables.Nat, chain, "-m", "addrtype", "--dst-type", "LOCAL", "-j", ingressChain) {
				if err := iptable.RawCombinedOutput("-t", "nat", "-I", chain, "-m", "addrtype", "--dst-type", "LOCAL", "-j", ingressChain); err != nil {
					return fmt.Errorf("failed to add jump rule in %s to ingress chain: %v", chain, err)
				}
			}
		}

		if !iptable.Exists(iptables.Filter, "FORWARD", "-j", ingressChain) {
			if err := iptable.RawCombinedOutput("-I", "FORWARD", "-j", ingressChain); err != nil {
				return fmt.Errorf("failed to add jump rule to %s in filter table forward chain: %v", ingressChain, err)
			}
			arrangeUserFilterRule()
		}

		oifName, err := findOIFName(gwIP)
		if err != nil {
			return fmt.Errorf("failed to find gateway bridge interface name for %s: %v", gwIP, err)
		}

		path := filepath.Join("/proc/sys/net/ipv4/conf", oifName, "route_localnet")
		if err := os.WriteFile(path, []byte{'1', '\n'}, 0644); err != nil { //nolint:gosec // gosec complains about perms here, which must be 0644 in this case
			return fmt.Errorf("could not write to %s: %v", path, err)
		}

		ruleArgs := []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", oifName, "-j", "MASQUERADE"}
		if !iptable.Exists(iptables.Nat, "POSTROUTING", ruleArgs...) {
			if err := iptable.RawCombinedOutput(append([]string{"-t", "nat", "-I", "POSTROUTING"}, ruleArgs...)...); err != nil {
				return fmt.Errorf("failed to add ingress localhost POSTROUTING rule for %s: %v", oifName, err)
			}
		}
	}

	// Filter the ingress ports until port rules start to be added/deleted
	filteredPorts := filterPortConfigs(ingressPorts, isDelete)
	rollbackRules := make([][]string, 0, len(filteredPorts)*3)
	var portErr error
	defer func() {
		if portErr != nil && !isDelete {
			filterPortConfigs(filteredPorts, !isDelete)
			for _, rule := range rollbackRules {
				if err := iptable.RawCombinedOutput(rule...); err != nil {
					logrus.Warnf("roll back rule failed, %v: %v", rule, err)
				}
			}
		}
	}()

	for _, iPort := range filteredPorts {
		var (
			protocol      = strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)])
			publishedPort = strconv.FormatUint(uint64(iPort.PublishedPort), 10)
			destination   = net.JoinHostPort(gwIP.String(), publishedPort)
		)
		if iptable.ExistChain(ingressChain, iptables.Nat) {
			rule := []string{"-t", "nat", addDelOpt, ingressChain, "-p", protocol, "--dport", publishedPort, "-j", "DNAT", "--to-destination", destination}

			if portErr = iptable.RawCombinedOutput(rule...); portErr != nil {
				err := fmt.Errorf("set up rule failed, %v: %v", rule, portErr)
				if !isDelete {
					return err
				}
				logrus.Info(err)
			}
			rollbackRule := []string{"-t", "nat", rollbackAddDelOpt, ingressChain, "-p", protocol, "--dport", publishedPort, "-j", "DNAT", "--to-destination", destination}
			rollbackRules = append(rollbackRules, rollbackRule)
		}

		// Filter table rules to allow a published service to be accessible in the local node from..
		// 1) service tasks attached to other networks
		// 2) unmanaged containers on bridge networks
		rule := []string{addDelOpt, ingressChain, "-m", "state", "-p", protocol, "--sport", publishedPort, "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"}
		if portErr = iptable.RawCombinedOutput(rule...); portErr != nil {
			err := fmt.Errorf("set up rule failed, %v: %v", rule, portErr)
			if !isDelete {
				return err
			}
			logrus.Warn(err)
		}
		rollbackRule := []string{rollbackAddDelOpt, ingressChain, "-m", "state", "-p", protocol, "--sport", publishedPort, "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"}
		rollbackRules = append(rollbackRules, rollbackRule)

		rule = []string{addDelOpt, ingressChain, "-p", protocol, "--dport", publishedPort, "-j", "ACCEPT"}
		if portErr = iptable.RawCombinedOutput(rule...); portErr != nil {
			err := fmt.Errorf("set up rule failed, %v: %v", rule, portErr)
			if !isDelete {
				return err
			}
			logrus.Warn(err)
		}
		rollbackRule = []string{rollbackAddDelOpt, ingressChain, "-p", protocol, "--dport", publishedPort, "-j", "ACCEPT"}
		rollbackRules = append(rollbackRules, rollbackRule)

		if err := plumbProxy(iPort, isDelete); err != nil {
			logrus.Warnf("failed to create proxy for port %s: %v", publishedPort, err)
		}
	}

	return nil
}

// In the filter table FORWARD chain the first rule should be to jump to
// DOCKER-USER so the user is able to filter packet first.
// The second rule should be jump to INGRESS-CHAIN.
// This chain has the rules to allow access to the published ports for swarm tasks
// from local bridge networks and docker_gwbridge (ie:taks on other swarm networks)
func arrangeIngressFilterRule() {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	if iptable.ExistChain(ingressChain, iptables.Filter) {
		if iptable.Exists(iptables.Filter, "FORWARD", "-j", ingressChain) {
			if err := iptable.RawCombinedOutput("-D", "FORWARD", "-j", ingressChain); err != nil {
				logrus.Warnf("failed to delete jump rule to ingressChain in filter table: %v", err)
			}
		}
		if err := iptable.RawCombinedOutput("-I", "FORWARD", "-j", ingressChain); err != nil {
			logrus.Warnf("failed to add jump rule to ingressChain in filter table: %v", err)
		}
	}
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

func plumbProxy(iPort *PortConfig, isDelete bool) error {
	var (
		err error
		l   io.Closer
	)

	portSpec := fmt.Sprintf("%d/%s", iPort.PublishedPort, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]))
	if isDelete {
		if listener, ok := ingressProxyTbl[portSpec]; ok {
			if listener != nil {
				listener.Close()
			}
		}

		return nil
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
		return err
	}

	ingressProxyTbl[portSpec] = l

	return nil
}

func writePortsToFile(ports []*PortConfig) (string, error) {
	f, err := os.CreateTemp("", "port_configs")
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:gosec

	buf, _ := proto.Marshal(&EndpointRecord{
		IngressPorts: ports,
	})

	n, err := f.Write(buf)
	if err != nil {
		return "", err
	}

	if n < len(buf) {
		return "", io.ErrShortWrite
	}

	return f.Name(), nil
}

func readPortsFromFile(fileName string) ([]*PortConfig, error) {
	buf, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var epRec EndpointRecord
	err = proto.Unmarshal(buf, &epRec)
	if err != nil {
		return nil, err
	}

	return epRec.IngressPorts, nil
}

// Invoke fwmarker reexec routine to mark vip destined packets with
// the passed firewall mark.
func invokeFWMarker(path string, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, eIP *net.IPNet, isDelete bool, lbMode string) error {
	var ingressPortsFile string

	if len(ingressPorts) != 0 {
		var err error
		ingressPortsFile, err = writePortsToFile(ingressPorts)
		if err != nil {
			return err
		}

		defer os.Remove(ingressPortsFile)
	}

	addDelOpt := "-A"
	if isDelete {
		addDelOpt = "-D"
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"fwmarker"}, path, vip.String(), strconv.FormatUint(uint64(fwMark), 10), addDelOpt, ingressPortsFile, eIP.String(), lbMode),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reexec failed: %v", err)
	}

	return nil
}

// Firewall marker reexec function.
func fwMarker() {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 8 {
		logrus.Error("invalid number of arguments..")
		os.Exit(1)
	}

	var ingressPorts []*PortConfig
	if os.Args[5] != "" {
		var err error
		ingressPorts, err = readPortsFromFile(os.Args[5])
		if err != nil {
			logrus.Errorf("Failed reading ingress ports file: %v", err)
			os.Exit(2)
		}
	}

	vip := os.Args[2]
	fwMark := os.Args[3]
	if _, err := strconv.ParseUint(fwMark, 10, 32); err != nil {
		logrus.Errorf("bad fwmark value(%s) passed: %v", fwMark, err)
		os.Exit(3)
	}
	addDelOpt := os.Args[4]

	rules := make([][]string, 0, len(ingressPorts))
	for _, iPort := range ingressPorts {
		var (
			protocol      = strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)])
			publishedPort = strconv.FormatUint(uint64(iPort.PublishedPort), 10)
		)
		rule := []string{"-t", "mangle", addDelOpt, "PREROUTING", "-p", protocol, "--dport", publishedPort, "-j", "MARK", "--set-mark", fwMark}
		rules = append(rules, rule)
	}

	ns, err := netns.GetFromPath(os.Args[1])
	if err != nil {
		logrus.Errorf("failed get network namespace %q: %v", os.Args[1], err)
		os.Exit(4)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		logrus.Errorf("setting into container net ns %v failed, %v", os.Args[1], err)
		os.Exit(5)
	}

	lbMode := os.Args[7]
	if addDelOpt == "-A" && lbMode == loadBalancerModeNAT {
		eIP, subnet, err := net.ParseCIDR(os.Args[6])
		if err != nil {
			logrus.Errorf("Failed to parse endpoint IP %s: %v", os.Args[6], err)
			os.Exit(6)
		}

		ruleParams := []string{"-m", "ipvs", "--ipvs", "-d", subnet.String(), "-j", "SNAT", "--to-source", eIP.String()}
		if !iptable.Exists("nat", "POSTROUTING", ruleParams...) {
			rule := append([]string{"-t", "nat", "-A", "POSTROUTING"}, ruleParams...)
			rules = append(rules, rule)

			err := os.WriteFile("/proc/sys/net/ipv4/vs/conntrack", []byte{'1', '\n'}, 0644)
			if err != nil {
				logrus.Errorf("Failed to write to /proc/sys/net/ipv4/vs/conntrack: %v", err)
				os.Exit(7)
			}
		}
	}

	rule := []string{"-t", "mangle", addDelOpt, "INPUT", "-d", vip + "/32", "-j", "MARK", "--set-mark", fwMark}
	rules = append(rules, rule)

	for _, rule := range rules {
		if err := iptable.RawCombinedOutputNative(rule...); err != nil {
			logrus.Errorf("set up rule failed, %v: %v", rule, err)
			os.Exit(8)
		}
	}
}

func addRedirectRules(path string, eIP *net.IPNet, ingressPorts []*PortConfig) error {
	var ingressPortsFile string

	if len(ingressPorts) != 0 {
		var err error
		ingressPortsFile, err = writePortsToFile(ingressPorts)
		if err != nil {
			return err
		}
		defer os.Remove(ingressPortsFile)
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"redirector"}, path, eIP.String(), ingressPortsFile),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reexec failed: %v", err)
	}

	return nil
}

// Redirector reexec function.
func redirector() {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 4 {
		logrus.Error("invalid number of arguments..")
		os.Exit(1)
	}

	var ingressPorts []*PortConfig
	if os.Args[3] != "" {
		var err error
		ingressPorts, err = readPortsFromFile(os.Args[3])
		if err != nil {
			logrus.Errorf("Failed reading ingress ports file: %v", err)
			os.Exit(2)
		}
	}

	eIP, _, err := net.ParseCIDR(os.Args[2])
	if err != nil {
		logrus.Errorf("Failed to parse endpoint IP %s: %v", os.Args[2], err)
		os.Exit(3)
	}
	ipAddr := eIP.String()

	rules := make([][]string, 0, len(ingressPorts)*3) // 3 rules per port
	for _, iPort := range ingressPorts {
		var (
			protocol      = strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)])
			publishedPort = strconv.FormatUint(uint64(iPort.PublishedPort), 10)
			targetPort    = strconv.FormatUint(uint64(iPort.TargetPort), 10)
		)

		rules = append(rules,
			[]string{"-t", "nat", "-A", "PREROUTING", "-d", ipAddr, "-p", protocol, "--dport", publishedPort, "-j", "REDIRECT", "--to-port", targetPort},

			// Allow only incoming connections to exposed ports
			[]string{"-I", "INPUT", "-d", ipAddr, "-p", protocol, "--dport", targetPort, "-m", "conntrack", "--ctstate", "NEW,ESTABLISHED", "-j", "ACCEPT"},

			// Allow only outgoing connections from exposed ports
			[]string{"-I", "OUTPUT", "-s", ipAddr, "-p", protocol, "--sport", targetPort, "-m", "conntrack", "--ctstate", "ESTABLISHED", "-j", "ACCEPT"},
		)
	}

	ns, err := netns.GetFromPath(os.Args[1])
	if err != nil {
		logrus.Errorf("failed get network namespace %q: %v", os.Args[1], err)
		os.Exit(4)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		logrus.Errorf("setting into container net ns %v failed, %v", os.Args[1], err)
		os.Exit(5)
	}

	for _, rule := range rules {
		if err := iptable.RawCombinedOutputNative(rule...); err != nil {
			logrus.Errorf("set up rule failed, %v: %v", rule, err)
			os.Exit(6)
		}
	}

	if len(ingressPorts) == 0 {
		return
	}

	// Ensure blocking rules for anything else in/to ingress network
	for _, rule := range [][]string{
		{"-d", ipAddr, "-p", "sctp", "-j", "DROP"},
		{"-d", ipAddr, "-p", "udp", "-j", "DROP"},
		{"-d", ipAddr, "-p", "tcp", "-j", "DROP"},
	} {
		if !iptable.ExistsNative(iptables.Filter, "INPUT", rule...) {
			if err := iptable.RawCombinedOutputNative(append([]string{"-A", "INPUT"}, rule...)...); err != nil {
				logrus.Errorf("set up rule failed, %v: %v", rule, err)
				os.Exit(7)
			}
		}
		rule[0] = "-s"
		if !iptable.ExistsNative(iptables.Filter, "OUTPUT", rule...) {
			if err := iptable.RawCombinedOutputNative(append([]string{"-A", "OUTPUT"}, rule...)...); err != nil {
				logrus.Errorf("set up rule failed, %v: %v", rule, err)
				os.Exit(8)
			}
		}
	}
}
