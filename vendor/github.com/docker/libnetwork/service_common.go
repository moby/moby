// +build linux windows

package libnetwork

import (
	"net"

	"github.com/Sirupsen/logrus"
)

func newService(name string, id string, ingressPorts []*PortConfig, aliases []string) *service {
	return &service{
		name:          name,
		id:            id,
		ingressPorts:  ingressPorts,
		loadBalancers: make(map[string]*loadBalancer),
		aliases:       aliases,
	}
}

func (c *controller) getLBIndex(sid, nid string, ingressPorts []*PortConfig) int {
	skey := serviceKey{
		id:    sid,
		ports: portConfigs(ingressPorts).String(),
	}
	c.Lock()
	s, ok := c.serviceBindings[skey]
	c.Unlock()

	if !ok {
		return 0
	}

	s.Lock()
	lb := s.loadBalancers[nid]
	s.Unlock()

	return int(lb.fwMark)
}

func (c *controller) cleanupServiceBindings(cleanupNID string) {
	var cleanupFuncs []func()

	c.Lock()
	services := make([]*service, 0, len(c.serviceBindings))
	for _, s := range c.serviceBindings {
		services = append(services, s)
	}
	c.Unlock()

	for _, s := range services {
		s.Lock()
		for nid, lb := range s.loadBalancers {
			if cleanupNID != "" && nid != cleanupNID {
				continue
			}

			for eid, ip := range lb.backEnds {
				service := s
				loadBalancer := lb
				networkID := nid
				epID := eid
				epIP := ip

				cleanupFuncs = append(cleanupFuncs, func() {
					if err := c.rmServiceBinding(service.name, service.id, networkID, epID, loadBalancer.vip,
						service.ingressPorts, service.aliases, epIP); err != nil {
						logrus.Errorf("Failed to remove service bindings for service %s network %s endpoint %s while cleanup: %v",
							service.id, networkID, epID, err)
					}
				})
			}
		}
		s.Unlock()
	}

	for _, f := range cleanupFuncs {
		f()
	}

}

func (c *controller) addServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, aliases []string, ip net.IP) error {
	n, err := c.NetworkByID(nid)
	if err != nil {
		return err
	}

	skey := serviceKey{
		id:    sid,
		ports: portConfigs(ingressPorts).String(),
	}

	c.Lock()
	s, ok := c.serviceBindings[skey]
	if !ok {
		// Create a new service if we are seeing this service
		// for the first time.
		s = newService(name, sid, ingressPorts, aliases)
		c.serviceBindings[skey] = s
	}
	c.Unlock()

	// Add endpoint IP to special "tasks.svc_name" so that the
	// applications have access to DNS RR.
	n.(*network).addSvcRecords("tasks."+name, ip, nil, false)
	for _, alias := range aliases {
		n.(*network).addSvcRecords("tasks."+alias, ip, nil, false)
	}

	// Add service name to vip in DNS, if vip is valid. Otherwise resort to DNS RR
	svcIP := vip
	if len(svcIP) == 0 {
		svcIP = ip
	}
	n.(*network).addSvcRecords(name, svcIP, nil, false)
	for _, alias := range aliases {
		n.(*network).addSvcRecords(alias, svcIP, nil, false)
	}

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
	}

	lb.backEnds[eid] = ip

	// Add loadbalancer service and backend in all sandboxes in
	// the network only if vip is valid.
	if len(vip) != 0 {
		n.(*network).addLBBackend(ip, vip, lb.fwMark, ingressPorts)
	}

	return nil
}

func (c *controller) rmServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, aliases []string, ip net.IP) error {
	var rmService bool

	n, err := c.NetworkByID(nid)
	if err != nil {
		return err
	}

	skey := serviceKey{
		id:    sid,
		ports: portConfigs(ingressPorts).String(),
	}

	c.Lock()
	s, ok := c.serviceBindings[skey]
	c.Unlock()
	if !ok {
		return nil
	}

	s.Lock()
	lb, ok := s.loadBalancers[nid]
	if !ok {
		s.Unlock()
		return nil
	}

	_, ok = lb.backEnds[eid]
	if !ok {
		s.Unlock()
		return nil
	}

	delete(lb.backEnds, eid)
	if len(lb.backEnds) == 0 {
		// All the backends for this service have been
		// removed. Time to remove the load balancer and also
		// remove the service entry in IPVS.
		rmService = true

		delete(s.loadBalancers, nid)
	}

	if len(s.loadBalancers) == 0 {
		// All loadbalancers for the service removed. Time to
		// remove the service itself.
		c.Lock()
		delete(c.serviceBindings, skey)
		c.Unlock()
	}

	// Remove loadbalancer service(if needed) and backend in all
	// sandboxes in the network only if the vip is valid.
	if len(vip) != 0 {
		n.(*network).rmLBBackend(ip, vip, lb.fwMark, ingressPorts, rmService)
	}
	s.Unlock()

	// Delete the special "tasks.svc_name" backend record.
	n.(*network).deleteSvcRecords("tasks."+name, ip, nil, false)
	for _, alias := range aliases {
		n.(*network).deleteSvcRecords("tasks."+alias, ip, nil, false)
	}

	// If we are doing DNS RR add the endpoint IP to DNS record
	// right away.
	if len(vip) == 0 {
		n.(*network).deleteSvcRecords(name, ip, nil, false)
		for _, alias := range aliases {
			n.(*network).deleteSvcRecords(alias, ip, nil, false)
		}
	}

	// Remove the DNS record for VIP only if we are removing the service
	if rmService && len(vip) != 0 {
		n.(*network).deleteSvcRecords(name, vip, nil, false)
		for _, alias := range aliases {
			n.(*network).deleteSvcRecords(alias, vip, nil, false)
		}
	}

	return nil
}
