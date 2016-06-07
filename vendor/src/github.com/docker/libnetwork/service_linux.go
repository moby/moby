package libnetwork

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/ipvs"
	"github.com/gogo/protobuf/proto"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("fwmarker", fwMarker)
}

func newService(name string, id string, ingressPorts []*PortConfig) *service {
	return &service{
		name:          name,
		id:            id,
		ingressPorts:  ingressPorts,
		loadBalancers: make(map[string]*loadBalancer),
	}
}

func (c *controller) addServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, ip net.IP) error {
	var (
		s          *service
		addService bool
	)

	n, err := c.NetworkByID(nid)
	if err != nil {
		return err
	}

	c.Lock()
	s, ok := c.serviceBindings[sid]
	if !ok {
		// Create a new service if we are seeing this service
		// for the first time.
		s = newService(name, sid, ingressPorts)
		c.serviceBindings[sid] = s
	}
	c.Unlock()

	// Add endpoint IP to special "tasks.svc_name" so that the
	// applications have access to DNS RR.
	n.(*network).addSvcRecords("tasks."+name, ip, nil, false)

	s.Lock()
	defer s.Unlock()

	lb, ok := s.loadBalancers[nid]
	if !ok {
		// Create a new load balancer if we are seeing this
		// network attachment on the service for the first
		// time.
		lb = &loadBalancer{
			vip:      vip,
			fwMark:   fwMarkCtr,
			backEnds: make(map[string]net.IP),
			service:  s,
		}

		fwMarkCtrMu.Lock()
		fwMarkCtr++
		fwMarkCtrMu.Unlock()

		s.loadBalancers[nid] = lb

		// Since we just created this load balancer make sure
		// we add a new service service in IPVS rules.
		addService = true

		// Add service name to vip in DNS, if vip is valid. Otherwise resort to DNS RR
		svcIP := vip
		if len(svcIP) == 0 {
			svcIP = ip
		}

		n.(*network).addSvcRecords(name, svcIP, nil, false)
	}

	lb.backEnds[eid] = ip

	// Add loadbalancer service and backend in all sandboxes in
	// the network only if vip is valid.
	if len(vip) != 0 {
		n.(*network).addLBBackend(ip, vip, lb.fwMark, ingressPorts, addService)
	}

	return nil
}

func (c *controller) rmServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, ip net.IP) error {
	var rmService bool

	n, err := c.NetworkByID(nid)
	if err != nil {
		return err
	}

	c.Lock()
	s, ok := c.serviceBindings[sid]
	if !ok {
		c.Unlock()
		return nil
	}
	c.Unlock()

	// Delete the special "tasks.svc_name" backend record.
	n.(*network).deleteSvcRecords("tasks."+name, ip, nil, false)

	s.Lock()
	defer s.Unlock()

	lb, ok := s.loadBalancers[nid]
	if !ok {
		return nil
	}

	delete(lb.backEnds, eid)
	if len(lb.backEnds) == 0 {
		// All the backends for this service have been
		// removed. Time to remove the load balancer and also
		// remove the service entry in IPVS.
		rmService = true

		// Make sure to remove the right IP since if vip is
		// not valid we would have added a DNS RR record.
		svcIP := vip
		if len(svcIP) == 0 {
			svcIP = ip
		}

		n.(*network).deleteSvcRecords(name, svcIP, nil, false)
		delete(s.loadBalancers, nid)
	}

	if len(s.loadBalancers) == 0 {
		// All loadbalancers for the service removed. Time to
		// remove the service itself.
		delete(c.serviceBindings, sid)
	}

	// Remove loadbalancer service(if needed) and backend in all
	// sandboxes in the network only if the vip is valid.
	if len(vip) != 0 {
		n.(*network).rmLBBackend(ip, vip, lb.fwMark, ingressPorts, rmService)
	}

	return nil
}

// Get all loadbalancers on this network that is currently discovered
// on this node.
func (n *network) connectedLoadbalancers() []*loadBalancer {
	c := n.getController()

	c.Lock()
	defer c.Unlock()

	var lbs []*loadBalancer
	for _, s := range c.serviceBindings {
		if lb, ok := s.loadBalancers[n.ID()]; ok {
			lbs = append(lbs, lb)
		}
	}

	return lbs
}

// Populate all loadbalancers on the network that the passed endpoint
// belongs to, into this sandbox.
func (sb *sandbox) populateLoadbalancers(ep *endpoint) {
	var gwIP net.IP

	n := ep.getNetwork()
	eIP := ep.Iface().Address()

	if sb.ingress {
		// For the ingress sandbox if this is not gateway
		// endpoint do nothing.
		if ep != sb.getGatewayEndpoint() {
			return
		}

		// This is the gateway endpoint. Now get the ingress
		// network and plumb the loadbalancers.
		gwIP = ep.Iface().Address().IP
		for _, ep := range sb.getConnectedEndpoints() {
			if !ep.endpointInGWNetwork() {
				n = ep.getNetwork()
				eIP = ep.Iface().Address()
			}
		}
	}

	for _, lb := range n.connectedLoadbalancers() {
		// Skip if vip is not valid.
		if len(lb.vip) == 0 {
			continue
		}

		addService := true
		for _, ip := range lb.backEnds {
			sb.addLBBackend(ip, lb.vip, lb.fwMark, lb.service.ingressPorts,
				eIP, gwIP, addService)
			addService = false
		}
	}
}

// Add loadbalancer backend to all sandboxes which has a connection to
// this network. If needed add the service as well, as specified by
// the addService bool.
func (n *network) addLBBackend(ip, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, addService bool) {
	n.WalkEndpoints(func(e Endpoint) bool {
		ep := e.(*endpoint)
		if sb, ok := ep.getSandbox(); ok {
			var gwIP net.IP
			if ep := sb.getGatewayEndpoint(); ep != nil {
				gwIP = ep.Iface().Address().IP
			}

			sb.addLBBackend(ip, vip, fwMark, ingressPorts, ep.Iface().Address(), gwIP, addService)
		}

		return false
	})
}

// Remove loadbalancer backend from all sandboxes which has a
// connection to this network. If needed remove the service entry as
// well, as specified by the rmService bool.
func (n *network) rmLBBackend(ip, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, rmService bool) {
	n.WalkEndpoints(func(e Endpoint) bool {
		ep := e.(*endpoint)
		if sb, ok := ep.getSandbox(); ok {
			var gwIP net.IP
			if ep := sb.getGatewayEndpoint(); ep != nil {
				gwIP = ep.Iface().Address().IP
			}

			sb.rmLBBackend(ip, vip, fwMark, ingressPorts, ep.Iface().Address(), gwIP, rmService)
		}

		return false
	})
}

// Add loadbalancer backend into one connected sandbox.
func (sb *sandbox) addLBBackend(ip, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, eIP *net.IPNet, gwIP net.IP, addService bool) {
	if sb.osSbox == nil {
		return
	}

	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create a ipvs handle for sbox %s: %v", sb.Key(), err)
		return
	}
	defer i.Close()

	s := &ipvs.Service{
		AddressFamily: nl.FAMILY_V4,
		FWMark:        fwMark,
		SchedName:     ipvs.RoundRobin,
	}

	if addService {
		var iPorts []*PortConfig
		if sb.ingress {
			iPorts = ingressPorts
			if err := programIngress(gwIP, iPorts, false); err != nil {
				logrus.Errorf("Failed to add ingress: %v", err)
				return
			}
		}

		logrus.Debugf("Creating service for vip %s fwMark %d ingressPorts %#v", vip, fwMark, iPorts)
		if err := invokeFWMarker(sb.Key(), vip, fwMark, iPorts, eIP, false); err != nil {
			logrus.Errorf("Failed to add firewall mark rule in sbox %s: %v", sb.Key(), err)
			return
		}

		if err := i.NewService(s); err != nil {
			logrus.Errorf("Failed to create a new service for vip %s fwmark %d: %v", vip, fwMark, err)
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
		logrus.Errorf("Failed to create real server %s for vip %s fwmark %d in sb %s: %v", ip, vip, fwMark, sb.containerID, err)
	}
}

// Remove loadbalancer backend from one connected sandbox.
func (sb *sandbox) rmLBBackend(ip, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, eIP *net.IPNet, gwIP net.IP, rmService bool) {
	if sb.osSbox == nil {
		return
	}

	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create a ipvs handle for sbox %s: %v", sb.Key(), err)
		return
	}
	defer i.Close()

	s := &ipvs.Service{
		AddressFamily: nl.FAMILY_V4,
		FWMark:        fwMark,
	}

	d := &ipvs.Destination{
		AddressFamily: nl.FAMILY_V4,
		Address:       ip,
		Weight:        1,
	}

	if err := i.DelDestination(s, d); err != nil {
		logrus.Errorf("Failed to delete real server %s for vip %s fwmark %d: %v", ip, vip, fwMark, err)
		return
	}

	if rmService {
		s.SchedName = ipvs.RoundRobin
		if err := i.DelService(s); err != nil {
			logrus.Errorf("Failed to create a new service for vip %s fwmark %d: %v", vip, fwMark, err)
			return
		}

		var iPorts []*PortConfig
		if sb.ingress {
			iPorts = ingressPorts
			if err := programIngress(gwIP, iPorts, true); err != nil {
				logrus.Errorf("Failed to delete ingress: %v", err)
				return
			}
		}

		if err := invokeFWMarker(sb.Key(), vip, fwMark, iPorts, eIP, true); err != nil {
			logrus.Errorf("Failed to add firewall mark rule in sbox %s: %v", sb.Key(), err)
			return
		}
	}
}

const ingressChain = "DOCKER-INGRESS"

var (
	ingressOnce     sync.Once
	ingressProxyMu  sync.Mutex
	ingressProxyTbl = make(map[string]io.Closer)
)

func programIngress(gwIP net.IP, ingressPorts []*PortConfig, isDelete bool) error {
	addDelOpt := "-I"
	if isDelete {
		addDelOpt = "-D"
	}

	chainExists := iptables.ExistChain(ingressChain, iptables.Nat)

	ingressOnce.Do(func() {
		if chainExists {
			// Flush ingress chain rules during init if it
			// exists. It might contain stale rules from
			// previous life.
			if err := iptables.RawCombinedOutput("-t", "nat", "-F", ingressChain); err != nil {
				logrus.Errorf("Could not flush ingress chain rules during init: %v", err)
			}
		}
	})

	if !isDelete {
		if !chainExists {
			if err := iptables.RawCombinedOutput("-t", "nat", "-N", ingressChain); err != nil {
				return fmt.Errorf("failed to create ingress chain: %v", err)
			}
		}

		if !iptables.Exists(iptables.Nat, ingressChain, "-j", "RETURN") {
			if err := iptables.RawCombinedOutput("-t", "nat", "-A", ingressChain, "-j", "RETURN"); err != nil {
				return fmt.Errorf("failed to add return rule in ingress chain: %v", err)
			}
		}

		for _, chain := range []string{"OUTPUT", "PREROUTING"} {
			if !iptables.Exists(iptables.Nat, chain, "-j", ingressChain) {
				if err := iptables.RawCombinedOutput("-t", "nat", "-I", chain, "-j", ingressChain); err != nil {
					return fmt.Errorf("failed to add jump rule in %s to ingress chain: %v", chain, err)
				}
			}
		}
	}

	for _, iPort := range ingressPorts {
		if iptables.ExistChain(ingressChain, iptables.Nat) {
			rule := strings.Fields(fmt.Sprintf("-t nat %s %s -p %s --dport %d -j DNAT --to-destination %s:%d",
				addDelOpt, ingressChain, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.NodePort, gwIP, iPort.NodePort))
			if err := iptables.RawCombinedOutput(rule...); err != nil {
				return fmt.Errorf("setting up rule failed, %v: %v", rule, err)
			}
		}

		if err := plumbProxy(iPort, isDelete); err != nil {
			return fmt.Errorf("failed to create proxy for port %d: %v", iPort.NodePort, err)
		}
	}

	return nil
}

func plumbProxy(iPort *PortConfig, isDelete bool) error {
	var (
		err error
		l   io.Closer
	)

	portSpec := fmt.Sprintf("%d/%s", iPort.NodePort, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]))
	if isDelete {
		ingressProxyMu.Lock()
		if listener, ok := ingressProxyTbl[portSpec]; ok {
			if listener != nil {
				listener.Close()
			}
		}
		ingressProxyMu.Unlock()

		return nil
	}

	switch iPort.Protocol {
	case ProtocolTCP:
		l, err = net.ListenTCP("tcp", &net.TCPAddr{Port: int(iPort.NodePort)})
	case ProtocolUDP:
		l, err = net.ListenUDP("udp", &net.UDPAddr{Port: int(iPort.NodePort)})
	}

	if err != nil {
		return err
	}

	ingressProxyMu.Lock()
	ingressProxyTbl[portSpec] = l
	ingressProxyMu.Unlock()

	return nil
}

// Invoke fwmarker reexec routine to mark vip destined packets with
// the passed firewall mark.
func invokeFWMarker(path string, vip net.IP, fwMark uint32, ingressPorts []*PortConfig, eIP *net.IPNet, isDelete bool) error {
	var ingressPortsFile string
	if len(ingressPorts) != 0 {
		f, err := ioutil.TempFile("", "port_configs")
		if err != nil {
			return err
		}

		buf, err := proto.Marshal(&EndpointRecord{
			IngressPorts: ingressPorts,
		})

		n, err := f.Write(buf)
		if err != nil {
			f.Close()
			return err
		}

		if n < len(buf) {
			f.Close()
			return io.ErrShortWrite
		}

		ingressPortsFile = f.Name()
		f.Close()
	}

	addDelOpt := "-A"
	if isDelete {
		addDelOpt = "-D"
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"fwmarker"}, path, vip.String(), fmt.Sprintf("%d", fwMark), addDelOpt, ingressPortsFile, eIP.IP.String()),
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
		buf, err := ioutil.ReadFile(os.Args[5])
		if err != nil {
			logrus.Errorf("Failed to read ports config file: %v", err)
			os.Exit(6)
		}

		var epRec EndpointRecord
		err = proto.Unmarshal(buf, &epRec)
		if err != nil {
			logrus.Errorf("Failed to unmarshal ports config data: %v", err)
			os.Exit(7)
		}

		ingressPorts = epRec.IngressPorts
	}

	vip := os.Args[2]
	fwMark, err := strconv.ParseUint(os.Args[3], 10, 32)
	if err != nil {
		logrus.Errorf("bad fwmark value(%s) passed: %v", os.Args[3], err)
		os.Exit(2)
	}
	addDelOpt := os.Args[4]

	rules := [][]string{}
	for _, iPort := range ingressPorts {
		rule := strings.Fields(fmt.Sprintf("-t nat %s PREROUTING -p %s --dport %d -j REDIRECT --to-port %d",
			addDelOpt, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.NodePort, iPort.Port))
		rules = append(rules, rule)

		rule = strings.Fields(fmt.Sprintf("-t mangle %s PREROUTING -p %s --dport %d -j MARK --set-mark %d",
			addDelOpt, strings.ToLower(PortConfig_Protocol_name[int32(iPort.Protocol)]), iPort.NodePort, fwMark))
		rules = append(rules, rule)
	}

	ns, err := netns.GetFromPath(os.Args[1])
	if err != nil {
		logrus.Errorf("failed get network namespace %q: %v", os.Args[1], err)
		os.Exit(3)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		logrus.Errorf("setting into container net ns %v failed, %v", os.Args[1], err)
		os.Exit(4)
	}

	if len(ingressPorts) != 0 && addDelOpt == "-A" {
		ruleParams := strings.Fields(fmt.Sprintf("-m ipvs --ipvs -j SNAT --to-source %s", os.Args[6]))
		if !iptables.Exists("nat", "POSTROUTING", ruleParams...) {
			rule := append(strings.Fields("-t nat -A POSTROUTING"), ruleParams...)
			rules = append(rules, rule)

			err := ioutil.WriteFile("/proc/sys/net/ipv4/vs/conntrack", []byte{'1', '\n'}, 0644)
			if err != nil {
				logrus.Errorf("Failed to write to /proc/sys/net/ipv4/vs/conntrack: %v", err)
				os.Exit(8)
			}
		}
	}

	rule := strings.Fields(fmt.Sprintf("-t mangle %s OUTPUT -d %s/32 -j MARK --set-mark %d", addDelOpt, vip, fwMark))
	rules = append(rules, rule)

	for _, rule := range rules {
		if err := iptables.RawCombinedOutputNative(rule...); err != nil {
			logrus.Errorf("setting up rule failed, %v: %v", rule, err)
			os.Exit(5)
		}
	}
}
