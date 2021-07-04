// +build linux windows

package libnetwork

import (
	"net"

	"github.com/docker/docker/libnetwork/internal/setmatrix"
	"github.com/sirupsen/logrus"
)

const maxSetStringLen = 350

func (c *controller) addEndpointNameResolution(svcName, svcID, nID, eID, containerName string, vip net.IP, serviceAliases, taskAliases []string, ip net.IP, addService bool, method string) error {
	n, err := c.NetworkByID(nID)
	if err != nil {
		return err
	}

	logrus.Debugf("addEndpointNameResolution %s %s add_service:%t sAliases:%v tAliases:%v", eID, svcName, addService, serviceAliases, taskAliases)

	// Add container resolution mappings
	if err := c.addContainerNameResolution(nID, eID, containerName, taskAliases, ip, method); err != nil {
		return err
	}

	serviceID := svcID
	if serviceID == "" {
		// This is the case of a normal container not part of a service
		serviceID = eID
	}

	// Add endpoint IP to special "tasks.svc_name" so that the applications have access to DNS RR.
	n.(*network).addSvcRecords(eID, "tasks."+svcName, serviceID, ip, nil, false, method)
	for _, alias := range serviceAliases {
		n.(*network).addSvcRecords(eID, "tasks."+alias, serviceID, ip, nil, false, method)
	}

	// Add service name to vip in DNS, if vip is valid. Otherwise resort to DNS RR
	if len(vip) == 0 {
		n.(*network).addSvcRecords(eID, svcName, serviceID, ip, nil, false, method)
		for _, alias := range serviceAliases {
			n.(*network).addSvcRecords(eID, alias, serviceID, ip, nil, false, method)
		}
	}

	if addService && len(vip) != 0 {
		n.(*network).addSvcRecords(eID, svcName, serviceID, vip, nil, false, method)
		for _, alias := range serviceAliases {
			n.(*network).addSvcRecords(eID, alias, serviceID, vip, nil, false, method)
		}
	}

	return nil
}

func (c *controller) addContainerNameResolution(nID, eID, containerName string, taskAliases []string, ip net.IP, method string) error {
	n, err := c.NetworkByID(nID)
	if err != nil {
		return err
	}
	logrus.Debugf("addContainerNameResolution %s %s", eID, containerName)

	// Add resolution for container name
	n.(*network).addSvcRecords(eID, containerName, eID, ip, nil, true, method)

	// Add resolution for taskaliases
	for _, alias := range taskAliases {
		n.(*network).addSvcRecords(eID, alias, eID, ip, nil, false, method)
	}

	return nil
}

func (c *controller) deleteEndpointNameResolution(svcName, svcID, nID, eID, containerName string, vip net.IP, serviceAliases, taskAliases []string, ip net.IP, rmService, multipleEntries bool, method string) error {
	n, err := c.NetworkByID(nID)
	if err != nil {
		return err
	}

	logrus.Debugf("deleteEndpointNameResolution %s %s rm_service:%t suppress:%t sAliases:%v tAliases:%v", eID, svcName, rmService, multipleEntries, serviceAliases, taskAliases)

	// Delete container resolution mappings
	if err := c.delContainerNameResolution(nID, eID, containerName, taskAliases, ip, method); err != nil {
		logrus.WithError(err).Warn("Error delting container from resolver")
	}

	serviceID := svcID
	if serviceID == "" {
		// This is the case of a normal container not part of a service
		serviceID = eID
	}

	// Delete the special "tasks.svc_name" backend record.
	if !multipleEntries {
		n.(*network).deleteSvcRecords(eID, "tasks."+svcName, serviceID, ip, nil, false, method)
		for _, alias := range serviceAliases {
			n.(*network).deleteSvcRecords(eID, "tasks."+alias, serviceID, ip, nil, false, method)
		}
	}

	// If we are doing DNS RR delete the endpoint IP from DNS record right away.
	if !multipleEntries && len(vip) == 0 {
		n.(*network).deleteSvcRecords(eID, svcName, serviceID, ip, nil, false, method)
		for _, alias := range serviceAliases {
			n.(*network).deleteSvcRecords(eID, alias, serviceID, ip, nil, false, method)
		}
	}

	// Remove the DNS record for VIP only if we are removing the service
	if rmService && len(vip) != 0 && !multipleEntries {
		n.(*network).deleteSvcRecords(eID, svcName, serviceID, vip, nil, false, method)
		for _, alias := range serviceAliases {
			n.(*network).deleteSvcRecords(eID, alias, serviceID, vip, nil, false, method)
		}
	}

	return nil
}

func (c *controller) delContainerNameResolution(nID, eID, containerName string, taskAliases []string, ip net.IP, method string) error {
	n, err := c.NetworkByID(nID)
	if err != nil {
		return err
	}
	logrus.Debugf("delContainerNameResolution %s %s", eID, containerName)

	// Delete resolution for container name
	n.(*network).deleteSvcRecords(eID, containerName, eID, ip, nil, true, method)

	// Delete resolution for taskaliases
	for _, alias := range taskAliases {
		n.(*network).deleteSvcRecords(eID, alias, eID, ip, nil, true, method)
	}

	return nil
}

func newService(name string, id string, ingressPorts []*PortConfig, serviceAliases []string) *service {
	return &service{
		name:          name,
		id:            id,
		ingressPorts:  ingressPorts,
		loadBalancers: make(map[string]*loadBalancer),
		aliases:       serviceAliases,
		ipToEndpoint:  setmatrix.NewSetMatrix(),
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

// cleanupServiceDiscovery when the network is being deleted, erase all the associated service discovery records
func (c *controller) cleanupServiceDiscovery(cleanupNID string) {
	c.Lock()
	defer c.Unlock()
	if cleanupNID == "" {
		logrus.Debugf("cleanupServiceDiscovery for all networks")
		c.svcRecords = make(map[string]svcInfo)
		return
	}
	logrus.Debugf("cleanupServiceDiscovery for network:%s", cleanupNID)
	delete(c.svcRecords, cleanupNID)
}

func (c *controller) cleanupServiceBindings(cleanupNID string) {
	var cleanupFuncs []func()

	logrus.Debugf("cleanupServiceBindings for %s", cleanupNID)
	c.Lock()
	services := make([]*service, 0, len(c.serviceBindings))
	for _, s := range c.serviceBindings {
		services = append(services, s)
	}
	c.Unlock()

	for _, s := range services {
		s.Lock()
		// Skip the serviceBindings that got deleted
		if s.deleted {
			s.Unlock()
			continue
		}
		for nid, lb := range s.loadBalancers {
			if cleanupNID != "" && nid != cleanupNID {
				continue
			}
			for eid, be := range lb.backEnds {
				cleanupFuncs = append(cleanupFuncs, makeServiceCleanupFunc(c, s, nid, eid, lb.vip, be.ip))
			}
		}
		s.Unlock()
	}

	for _, f := range cleanupFuncs {
		f()
	}

}

func makeServiceCleanupFunc(c *controller, s *service, nID, eID string, vip net.IP, ip net.IP) func() {
	// ContainerName and taskAliases are not available here, this is still fine because the Service discovery
	// cleanup already happened before. The only thing that rmServiceBinding is still doing here a part from the Load
	// Balancer bookeeping, is to keep consistent the mapping of endpoint to IP.
	return func() {
		if err := c.rmServiceBinding(s.name, s.id, nID, eID, "", vip, s.ingressPorts, s.aliases, []string{}, ip, "cleanupServiceBindings", false, true); err != nil {
			logrus.Errorf("Failed to remove service bindings for service %s network %s endpoint %s while cleanup: %v", s.id, nID, eID, err)
		}
	}
}

func (c *controller) addServiceBinding(svcName, svcID, nID, eID, containerName string, vip net.IP, ingressPorts []*PortConfig, serviceAliases, taskAliases []string, ip net.IP, method string) error {
	var addService bool

	// Failure to lock the network ID on add can result in racing
	// racing against network deletion resulting in inconsistent
	// state in the c.serviceBindings map and it's sub-maps. Also,
	// always lock network ID before services to avoid deadlock.
	c.networkLocker.Lock(nID)
	defer c.networkLocker.Unlock(nID) // nolint:errcheck

	n, err := c.NetworkByID(nID)
	if err != nil {
		return err
	}

	skey := serviceKey{
		id:    svcID,
		ports: portConfigs(ingressPorts).String(),
	}

	var s *service
	for {
		c.Lock()
		var ok bool
		s, ok = c.serviceBindings[skey]
		if !ok {
			// Create a new service if we are seeing this service
			// for the first time.
			s = newService(svcName, svcID, ingressPorts, serviceAliases)
			c.serviceBindings[skey] = s
		}
		c.Unlock()
		s.Lock()
		if !s.deleted {
			// ok the object is good to be used
			break
		}
		s.Unlock()
	}
	logrus.Debugf("addServiceBinding from %s START for %s %s p:%p nid:%s skey:%v", method, svcName, eID, s, nID, skey)
	defer s.Unlock()

	lb, ok := s.loadBalancers[nID]
	if !ok {
		// Create a new load balancer if we are seeing this
		// network attachment on the service for the first
		// time.
		fwMarkCtrMu.Lock()

		lb = &loadBalancer{
			vip:      vip,
			fwMark:   fwMarkCtr,
			backEnds: make(map[string]*lbBackend),
			service:  s,
		}

		fwMarkCtr++
		fwMarkCtrMu.Unlock()

		s.loadBalancers[nID] = lb
		addService = true
	}

	lb.backEnds[eID] = &lbBackend{ip, false}

	ok, entries := s.assignIPToEndpoint(ip.String(), eID)
	if !ok || entries > 1 {
		setStr, b := s.printIPToEndpoint(ip.String())
		if len(setStr) > maxSetStringLen {
			setStr = setStr[:maxSetStringLen]
		}
		logrus.Warnf("addServiceBinding %s possible transient state ok:%t entries:%d set:%t %s", eID, ok, entries, b, setStr)
	}

	// Add loadbalancer service and backend to the network
	n.(*network).addLBBackend(ip, lb)

	// Add the appropriate name resolutions
	if err := c.addEndpointNameResolution(svcName, svcID, nID, eID, containerName, vip, serviceAliases, taskAliases, ip, addService, "addServiceBinding"); err != nil {
		return err
	}

	logrus.Debugf("addServiceBinding from %s END for %s %s", method, svcName, eID)

	return nil
}

func (c *controller) rmServiceBinding(svcName, svcID, nID, eID, containerName string, vip net.IP, ingressPorts []*PortConfig, serviceAliases []string, taskAliases []string, ip net.IP, method string, deleteSvcRecords bool, fullRemove bool) error {

	var rmService bool

	skey := serviceKey{
		id:    svcID,
		ports: portConfigs(ingressPorts).String(),
	}

	c.Lock()
	s, ok := c.serviceBindings[skey]
	c.Unlock()
	if !ok {
		logrus.Warnf("rmServiceBinding %s %s %s aborted c.serviceBindings[skey] !ok", method, svcName, eID)
		return nil
	}

	s.Lock()
	defer s.Unlock()
	logrus.Debugf("rmServiceBinding from %s START for %s %s p:%p nid:%s sKey:%v deleteSvc:%t", method, svcName, eID, s, nID, skey, deleteSvcRecords)
	lb, ok := s.loadBalancers[nID]
	if !ok {
		logrus.Warnf("rmServiceBinding %s %s %s aborted s.loadBalancers[nid] !ok", method, svcName, eID)
		return nil
	}

	be, ok := lb.backEnds[eID]
	if !ok {
		logrus.Warnf("rmServiceBinding %s %s %s aborted lb.backEnds[eid] && lb.disabled[eid] !ok", method, svcName, eID)
		return nil
	}

	if fullRemove {
		// delete regardless
		delete(lb.backEnds, eID)
	} else {
		be.disabled = true
	}

	if len(lb.backEnds) == 0 {
		// All the backends for this service have been
		// removed. Time to remove the load balancer and also
		// remove the service entry in IPVS.
		rmService = true

		delete(s.loadBalancers, nID)
		logrus.Debugf("rmServiceBinding %s delete %s, p:%p in loadbalancers len:%d", eID, nID, lb, len(s.loadBalancers))
	}

	ok, entries := s.removeIPToEndpoint(ip.String(), eID)
	if !ok || entries > 0 {
		setStr, b := s.printIPToEndpoint(ip.String())
		if len(setStr) > maxSetStringLen {
			setStr = setStr[:maxSetStringLen]
		}
		logrus.Warnf("rmServiceBinding %s possible transient state ok:%t entries:%d set:%t %s", eID, ok, entries, b, setStr)
	}

	// Remove loadbalancer service(if needed) and backend in all
	// sandboxes in the network only if the vip is valid.
	if entries == 0 {
		// The network may well have been deleted before the last
		// of the service bindings.  That's ok on Linux because
		// removing the network sandbox implicitly removes the
		// backend service bindings.  Windows VFP cleanup requires
		// calling cleanupServiceBindings on the network prior to
		// deleting the network, performed by network.delete.
		n, err := c.NetworkByID(nID)
		if err == nil {
			n.(*network).rmLBBackend(ip, lb, rmService, fullRemove)
		}
	}

	// Delete the name resolutions
	if deleteSvcRecords {
		if err := c.deleteEndpointNameResolution(svcName, svcID, nID, eID, containerName, vip, serviceAliases, taskAliases, ip, rmService, entries > 0, "rmServiceBinding"); err != nil {
			return err
		}
	}

	if len(s.loadBalancers) == 0 {
		// All loadbalancers for the service removed. Time to
		// remove the service itself.
		c.Lock()

		// Mark the object as deleted so that the add won't use it wrongly
		s.deleted = true
		// NOTE The delete from the serviceBindings map has to be the last operation else we are allowing a race between this service
		// that is getting deleted and a new service that will be created if the entry is not anymore there
		delete(c.serviceBindings, skey)
		c.Unlock()
	}

	logrus.Debugf("rmServiceBinding from %s END for %s %s", method, svcName, eID)
	return nil
}
