// +build !linux

package libnetwork

import (
	"fmt"
	"net"
)

func (c *controller) addServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, ip net.IP) error {
	return fmt.Errorf("not supported")
}

func (c *controller) rmServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, ip net.IP) error {
	return fmt.Errorf("not supported")
}

func (sb *sandbox) populateLoadbalancers(ep *endpoint) {
}
