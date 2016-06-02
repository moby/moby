package libnetwork

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/ipvs"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("fwmarker", fwMarker)
}

func newService(name string, id string) *service {
	return &service{
		name:          name,
		id:            id,
		loadBalancers: make(map[string]*loadBalancer),
	}
}

func (c *controller) addServiceBinding(name, sid, nid, eid string, vip net.IP, ip net.IP) error {
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
		s = newService(name, sid)
		c.serviceBindings[sid] = s
	}
	c.Unlock()

	s.Lock()
	lb, ok := s.loadBalancers[nid]
	if !ok {
		// Create a new load balancer if we are seeing this
		// network attachment on the service for the first
		// time.
		lb = &loadBalancer{
			vip:      vip,
			fwMark:   fwMarkCtr,
			backEnds: make(map[string]net.IP),
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
	s.Unlock()

	// Add endpoint IP to special "tasks.svc_name" so that the
	// applications have access to DNS RR.
	n.(*network).addSvcRecords("tasks."+name, ip, nil, false)

	// Add loadbalancer service and backend in all sandboxes in
	// the network only if vip is valid.
	if len(vip) != 0 {
		n.(*network).addLBBackend(ip, vip, lb.fwMark, addService)
	}

	return nil
}

func (c *controller) rmServiceBinding(name, sid, nid, eid string, vip net.IP, ip net.IP) error {
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

	s.Lock()
	lb, ok := s.loadBalancers[nid]
	if !ok {
		s.Unlock()
		return nil
	}

	// Delete the special "tasks.svc_name" backend record.
	n.(*network).deleteSvcRecords("tasks."+name, ip, nil, false)
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
	s.Unlock()

	// Remove loadbalancer service(if needed) and backend in all
	// sandboxes in the network only if the vip is valid.
	if len(vip) != 0 {
		n.(*network).rmLBBackend(ip, vip, lb.fwMark, rmService)
	}

	return nil
}

// Get all loadbalancers on this network that is currently discovered
// on this node..
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
	n := ep.getNetwork()
	for _, lb := range n.connectedLoadbalancers() {
		// Skip if vip is not valid.
		if len(lb.vip) == 0 {
			continue
		}

		addService := true
		for _, ip := range lb.backEnds {
			sb.addLBBackend(ip, lb.vip, lb.fwMark, addService)
			addService = false
		}
	}
}

// Add loadbalancer backend to all sandboxes which has a connection to
// this network. If needed add the service as well, as specified by
// the addService bool.
func (n *network) addLBBackend(ip, vip net.IP, fwMark uint32, addService bool) {
	n.WalkEndpoints(func(e Endpoint) bool {
		ep := e.(*endpoint)
		if sb, ok := ep.getSandbox(); ok {
			sb.addLBBackend(ip, vip, fwMark, addService)
		}

		return false
	})
}

// Remove loadbalancer backend from all sandboxes which has a
// connection to this network. If needed remove the service entry as
// well, as specified by the rmService bool.
func (n *network) rmLBBackend(ip, vip net.IP, fwMark uint32, rmService bool) {
	n.WalkEndpoints(func(e Endpoint) bool {
		ep := e.(*endpoint)
		if sb, ok := ep.getSandbox(); ok {
			sb.rmLBBackend(ip, vip, fwMark, rmService)
		}

		return false
	})
}

// Add loadbalancer backend into one connected sandbox.
func (sb *sandbox) addLBBackend(ip, vip net.IP, fwMark uint32, addService bool) {
	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create an ipvs handle for sbox %s: %v", sb.Key(), err)
		return
	}
	defer i.Close()

	s := &ipvs.Service{
		AddressFamily: nl.FAMILY_V4,
		FWMark:        fwMark,
		SchedName:     ipvs.RoundRobin,
	}

	if addService {
		logrus.Debugf("Creating service for vip %s fwMark %d", vip, fwMark)
		if err := invokeFWMarker(sb.Key(), vip, fwMark, false); err != nil {
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
	if err := i.NewDestination(s, d); err != nil {
		logrus.Errorf("Failed to create real server %s for vip %s fwmark %d: %v", ip, vip, fwMark, err)
	}
}

// Remove loadbalancer backend from one connected sandbox.
func (sb *sandbox) rmLBBackend(ip, vip net.IP, fwMark uint32, rmService bool) {
	i, err := ipvs.New(sb.Key())
	if err != nil {
		logrus.Errorf("Failed to create an ipvs handle for sbox %s: %v", sb.Key(), err)
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

		if err := invokeFWMarker(sb.Key(), vip, fwMark, true); err != nil {
			logrus.Errorf("Failed to add firewall mark rule in sbox %s: %v", sb.Key(), err)
			return
		}
	}
}

// Invoke fwmarker reexec routine to mark vip destined packets with
// the passed firewall mark.
func invokeFWMarker(path string, vip net.IP, fwMark uint32, isDelete bool) error {
	addDelOpt := "-A"
	if isDelete {
		addDelOpt = "-D"
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"fwmarker"}, path, vip.String(), fmt.Sprintf("%d", fwMark), addDelOpt),
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

	if len(os.Args) < 5 {
		logrus.Error("invalid number of arguments..")
		os.Exit(1)
	}

	vip := os.Args[2]
	fwMark, err := strconv.ParseUint(os.Args[3], 10, 32)
	if err != nil {
		logrus.Errorf("bad fwmark value(%s) passed: %v", os.Args[3], err)
		os.Exit(2)
	}
	addDelOpt := os.Args[4]

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

	rule := strings.Fields(fmt.Sprintf("-t mangle %s OUTPUT -d %s/32 -j MARK --set-mark %d", addDelOpt, vip, fwMark))
	if err := iptables.RawCombinedOutputNative(rule...); err != nil {
		logrus.Errorf("setting up rule failed, %v: %v", rule, err)
		os.Exit(5)
	}
}
