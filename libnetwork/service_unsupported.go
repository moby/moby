//go:build !linux && !windows
// +build !linux,!windows

package libnetwork

import (
	"fmt"
	"net"
)

func (c *Controller) cleanupServiceBindings(nid string) {
}

func (c *Controller) addServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, aliases []string, ip net.IP) error {
	return fmt.Errorf("not supported")
}

func (c *Controller) rmServiceBinding(name, sid, nid, eid string, vip net.IP, ingressPorts []*PortConfig, aliases []string, ip net.IP) error {
	return fmt.Errorf("not supported")
}

func (sb *sandbox) populateLoadBalancers(ep *endpoint) {
}

func arrangeIngressFilterRule() {
}
