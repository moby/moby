package libnetwork

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/ipvs"
	"github.com/docker/libnetwork/ns"
	"github.com/gogo/protobuf/proto"
	"github.com/ishidawataru/sctp"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("fwmarker", fwMarker)
	reexec.Register("redirecter", redirecter)
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
	ep = sb.getEndpoint(ep.ID())
	if ep == nil {
		return nil, nil, fmt.Errorf("Load balancing endpoint %s(%s) removed from %s", ep.Name(), ep.ID(), n.ID())
	}
	return ep, sb, nil
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
			filteredPorts := filterPortConfigs(lb.service.ingressPorts, false)
			if err := programIngress(gwIP, filteredPorts, false); err != nil {
				logrus.Errorf("Failed to add ingress: %v", err)
				return
			}
		}

		logrus.Debugf("Creating service for vip %s fwMark %d ingressPorts %#v in sbox %.7s (%.7s)", lb.vip, lb.fwMark, lb.service.ingressPorts, sb.ID(), sb.ContainerID())
		if err := invokeFWMarker(sb.Key(), lb.vip, lb.fwMark, lb.service.ingressPorts, eIP, false); err != nil {
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

	// Remove the sched name before using the service to add
	// destination.
	s.SchedName = ""
	if err := i.NewDestination(s, d); err != nil && err != syscall.EEXIST {
		logrus.Errorf("Failed to create real server %s for vip %s fwmark %d in sbox %.7s (%.7s): %v", ip, lb.vip, lb.fwMark, sb.ID(), sb.ContainerID(), err)
	}
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
			filteredPorts := filterPortConfigs(lb.service.ingressPorts, true)
			if err := programIngress(gwIP, filteredPorts, true); err != nil {
				logrus.Errorf("Failed to delete ingress: %v", err)
			}
		}

		if err := invokeFWMarker(sb.Key(), lb.vip, lb.fwMark, lb.service.ingressPorts, eIP, true); err != nil {
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
	addDelOpt := "-I"
	if isDelete {
		addDelOpt = "-D"
	}

	ingressMu.Lock()
	defer ingressMu.Unlock()

	chainExists := iptables.ExistChain(ingressChain, iptables.Nat)
	filterChainExists := iptables.ExistChain(ingressChain, iptables.Filter)

	ingressOnce.Do(func() {
		// Flush nat table and filter table ingress chain rules during init if it
		// exists. It might contain stale rules from previous life.
		if chainExists {
			if err := iptables.RawCombinedOutput("-t", "nat", "-F", ingressChain); err != nil {
				logrus.Errorf("Could not flush nat table ingress chain rules during init: %v", err)
			}
		}
		if filterChainExists {
			if err := iptables.RawCombinedOutput("-F", ingressChain); err != nil {
				logrus.Errorf("Could not flush filter table ingress chain rules during init: %v", err)
			}
		}
	})

	if !isDelete {
		if !chainExists {
			if err := iptables.RawCombinedOutput("-t", "nat", "-N", ingressChain); err != nil {
				return fmt.Errorf("failed to create ingress chain: %v", err)
			}
		}
		if !filterChainExists {
			if err := iptables.RawCombinedOutput("-N", ingressChain); err != nil {
				return fmt.Errorf("failed to create filter table ingress chain: %v", err)
			}
		}

		if !iptables.Exists(iptables.Nat, ingressChain, "-j", "RETURN") {
			if err := iptables.RawCombinedOutput("-t", "nat", "-A", ingressChain, "-j", "RETURN"); err != nil {
				return fmt.Errorf("failed to add return rule in nat table ingress chain: %v", err)
			}
		}

		if !iptables.Exists(iptables.Filter, ingressChain, "-j", "RETURN") {
			if err := iptables.RawCombinedOutput("-A", ingressChain, "-j", "RETURN"); err != nil {
				return fmt.Errorf("failed to add return rule to filter table ingress chain: %v", err)
			}
		}

		for _, chain := range []string{"OUTPUT", "PREROUTING"} {
			if !iptables.Exists(iptables.Nat, chain, "-m", "addrtype", "--dst-type", "LOCAL", "-j", ingressChain) {
				if err := iptables.RawCombinedOutput("-t", "nat", "-I", chain, "-m", "addrtype", "--dst-type", "LOCAL", "-j", ingressChain); err != nil {
					return fmt.Errorf("failed to add jump rule in %s to ingress chain: %v", chain, err)
				}
			}
		}

		if !iptables.Exists(iptables.Filter, "FORWARD", "-j", ingressChain) {
			if err := iptables.RawCombinedOutput("-I", "FORWARD", "-j", ingressChain); err != nil {
				return fmt.Errorf("failed to add jump rule to %s in filter table forward chain: %v", ingressChain, err)
			}
			arrangeUserFilterRule()
		}

		oifName, err := findOIFName(gwIP)
		if err != nil {
			return fmt.Errorf("failed to find gateway bridge interface name for %s: %v", gwIP, err)
		}

		path := filepath.Join("/proc/sys/net/ipv4/conf", oifName, "route_localnet")
		if err := ioutil.WriteFile(path, []byte{'1', '\n'}, 0644); err != nil {
			return fmt.Errorf("could not write to %s: %v", path, err)
		}

		ruleArgs := strings.Fields(fmt.Sprintf("-m addrtype --src-type LOCAL -o %s -j MASQUERADE", oifName))
		if !iptables.Exists(iptables.Nat, "POSTROUTING", ruleArgs...) {
			if err := iptables.RawCombinedOutput(append([]string{"-t", "nat", "-I", "POSTROUTING"}, ruleArgs...)...); err != nil {
				return fmt.Errorf("failed to add ingress localhost POSTROUTING rule for %s: %v", oifName, err)
			}
		}
	}

	for _, iPort := range ingressPorts {
		if iptables.ExistChain(ingressChain, iptables.Nat) {
			rule := strings.Fields(fmt.Sprintf("-t nat %s %s -p %s --dport %d -j DNAT --to-destination %s:%d",
				addDelOpt, ingressChain, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.PublishedPort, gwIP, iPort.PublishedPort))
			if err := iptables.RawCombinedOutput(rule...); err != nil {
				errStr := fmt.Sprintf("setting up rule failed, %v: %v", rule, err)
				if !isDelete {
					return fmt.Errorf("%s", errStr)
				}

				logrus.Infof("%s", errStr)
			}
		}

		// Filter table rules to allow a published service to be accessible in the local node from..
		// 1) service tasks attached to other networks
		// 2) unmanaged containers on bridge networks
		rule := strings.Fields(fmt.Sprintf("%s %s -m state -p %s --sport %d --state ESTABLISHED,RELATED -j ACCEPT",
			addDelOpt, ingressChain, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.PublishedPort))
		if err := iptables.RawCombinedOutput(rule...); err != nil {
			errStr := fmt.Sprintf("setting up rule failed, %v: %v", rule, err)
			if !isDelete {
				return fmt.Errorf("%s", errStr)
			}
			logrus.Warnf("%s", errStr)
		}

		rule = strings.Fields(fmt.Sprintf("%s %s -p %s --dport %d -j ACCEPT",
			addDelOpt, ingressChain, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.PublishedPort))
		if err := iptables.RawCombinedOutput(rule...); err != nil {
			errStr := fmt.Sprintf("setting up rule failed, %v: %v", rule, err)
			if !isDelete {
				return fmt.Errorf("%s", errStr)
			}

			logrus.Warnf("%s", errStr)
		}

		if err := plumbProxy(iPort, isDelete); err != nil {
			logrus.Warnf("failed to create proxy for port %d: %v", iPort.PublishedPort, err)
		}
	}

	return nil
}

// In the filter table FORWARD chain the first rule should be to jump to
// DOCKER-USER so the user is able to filter packet first.
// The second rule should be jump to INGRESS-CHAIN.
// This chain has the rules to allow access to the published ports for swarm tasks
// from local bridge networks and docker_gwbridge (ie:taks on other swarm netwroks)
func arrangeIngressFilterRule() {
	if iptables.ExistChain(ingressChain, iptables.Filter) {
		if iptables.Exists(iptables.Filter, "FORWARD", "-j", ingressChain) {
			if err := iptables.RawCombinedOutput("-D", "FORWARD", "-j", ingressChain); err != nil {
				logrus.Warnf("failed to delete jump rule to ingressChain in filter table: %v", err)
			}
		}
		if err := iptables.RawCombinedOutput("-I", "FORWARD", "-j", ingressChain); err != nil {
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
	f, err := ioutil.TempFile("", "port_configs")
	if err != nil {
		return "", err
	}
	defer f.Close()

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
	buf, err := ioutil.ReadFile(fileName)
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
func invokeFWMarker(path string, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, eIP *net.IPNet, isDelete bool) error {
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
		Args:   append([]string{"fwmarker"}, path, vip.String(), fmt.Sprintf("%d", fwMark), addDelOpt, ingressPortsFile, eIP.String()),
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
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 7 {
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
	fwMark, err := strconv.ParseUint(os.Args[3], 10, 32)
	if err != nil {
		logrus.Errorf("bad fwmark value(%s) passed: %v", os.Args[3], err)
		os.Exit(3)
	}
	addDelOpt := os.Args[4]

	rules := [][]string{}
	for _, iPort := range ingressPorts {
		rule := strings.Fields(fmt.Sprintf("-t mangle %s PREROUTING -p %s --dport %d -j MARK --set-mark %d",
			addDelOpt, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.PublishedPort, fwMark))
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

	if addDelOpt == "-A" {
		eIP, subnet, err := net.ParseCIDR(os.Args[6])
		if err != nil {
			logrus.Errorf("Failed to parse endpoint IP %s: %v", os.Args[6], err)
			os.Exit(6)
		}

		ruleParams := strings.Fields(fmt.Sprintf("-m ipvs --ipvs -d %s -j SNAT --to-source %s", subnet, eIP))
		if !iptables.Exists("nat", "POSTROUTING", ruleParams...) {
			rule := append(strings.Fields("-t nat -A POSTROUTING"), ruleParams...)
			rules = append(rules, rule)

			err := ioutil.WriteFile("/proc/sys/net/ipv4/vs/conntrack", []byte{'1', '\n'}, 0644)
			if err != nil {
				logrus.Errorf("Failed to write to /proc/sys/net/ipv4/vs/conntrack: %v", err)
				os.Exit(7)
			}
		}
	}

	rule := strings.Fields(fmt.Sprintf("-t mangle %s INPUT -d %s/32 -j MARK --set-mark %d", addDelOpt, vip, fwMark))
	rules = append(rules, rule)

	for _, rule := range rules {
		if err := iptables.RawCombinedOutputNative(rule...); err != nil {
			logrus.Errorf("setting up rule failed, %v: %v", rule, err)
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
		Args:   append([]string{"redirecter"}, path, eIP.String(), ingressPortsFile),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reexec failed: %v", err)
	}

	return nil
}

// Redirecter reexec function.
func redirecter() {
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

	rules := [][]string{}
	for _, iPort := range ingressPorts {
		rule := strings.Fields(fmt.Sprintf("-t nat -A PREROUTING -d %s -p %s --dport %d -j REDIRECT --to-port %d",
			eIP.String(), strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.PublishedPort, iPort.TargetPort))
		rules = append(rules, rule)
		// Allow only incoming connections to exposed ports
		iRule := strings.Fields(fmt.Sprintf("-I INPUT -d %s -p %s --dport %d -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT",
			eIP.String(), strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.TargetPort))
		rules = append(rules, iRule)
		// Allow only outgoing connections from exposed ports
		oRule := strings.Fields(fmt.Sprintf("-I OUTPUT -s %s -p %s --sport %d -m conntrack --ctstate ESTABLISHED -j ACCEPT",
			eIP.String(), strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.TargetPort))
		rules = append(rules, oRule)
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
		if err := iptables.RawCombinedOutputNative(rule...); err != nil {
			logrus.Errorf("setting up rule failed, %v: %v", rule, err)
			os.Exit(6)
		}
	}

	if len(ingressPorts) == 0 {
		return
	}

	// Ensure blocking rules for anything else in/to ingress network
	for _, rule := range [][]string{
		{"-d", eIP.String(), "-p", "sctp", "-j", "DROP"},
		{"-d", eIP.String(), "-p", "udp", "-j", "DROP"},
		{"-d", eIP.String(), "-p", "tcp", "-j", "DROP"},
	} {
		if !iptables.ExistsNative(iptables.Filter, "INPUT", rule...) {
			if err := iptables.RawCombinedOutputNative(append([]string{"-A", "INPUT"}, rule...)...); err != nil {
				logrus.Errorf("setting up rule failed, %v: %v", rule, err)
				os.Exit(7)
			}
		}
		rule[0] = "-s"
		if !iptables.ExistsNative(iptables.Filter, "OUTPUT", rule...) {
			if err := iptables.RawCombinedOutputNative(append([]string{"-A", "OUTPUT"}, rule...)...); err != nil {
				logrus.Errorf("setting up rule failed, %v: %v", rule, err)
				os.Exit(8)
			}
		}
	}
}
