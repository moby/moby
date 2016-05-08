package libnetwork

import "net"

type service struct {
	name     string
	id       string
	backEnds map[string]map[string]net.IP
}

func newService(name string, id string) *service {
	return &service{
		name:     name,
		id:       id,
		backEnds: make(map[string]map[string]net.IP),
	}
}

func (c *controller) addServiceBinding(name, sid, nid, eid string, ip net.IP) error {
	var s *service

	n, err := c.NetworkByID(nid)
	if err != nil {
		return err
	}

	c.Lock()
	s, ok := c.serviceBindings[sid]
	if !ok {
		s = newService(name, sid)
	}

	netBackEnds, ok := s.backEnds[nid]
	if !ok {
		netBackEnds = make(map[string]net.IP)
		s.backEnds[nid] = netBackEnds
	}

	netBackEnds[eid] = ip
	c.serviceBindings[sid] = s
	c.Unlock()

	n.(*network).addSvcRecords(name, ip, nil, false)
	return nil
}

func (c *controller) rmServiceBinding(name, sid, nid, eid string, ip net.IP) error {
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

	netBackEnds, ok := s.backEnds[nid]
	if !ok {
		c.Unlock()
		return nil
	}

	delete(netBackEnds, eid)

	if len(netBackEnds) == 0 {
		delete(s.backEnds, nid)
	}

	if len(s.backEnds) == 0 {
		delete(c.serviceBindings, sid)
	}
	c.Unlock()

	n.(*network).deleteSvcRecords(name, ip, nil, false)

	return err
}
