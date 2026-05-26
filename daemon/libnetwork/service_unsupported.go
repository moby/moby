//go:build !linux && !windows

package libnetwork

import (
	"net"

	"github.com/moby/moby/v2/errdefs"
)

func (c *Controller) cleanupServiceDiscovery(cleanupNID string) {}

func (c *Controller) cleanupServiceBindings(nid string) {}

func (c *Controller) getLBIndex(_, _ string, _ []*PortConfig) int {
	return 0
}

func (c *Controller) addContainerNameResolution(nID, eID, containerName string, taskAliases []string, ip net.IP, method string) error {
	return errdefs.PlatformNotImplemented{Feature: "Controller.addContainerNameResolution"}
}

func (c *Controller) delContainerNameResolution(nID, eID, containerName string, taskAliases []string, ip net.IP, method string) error {
	return errdefs.PlatformNotImplemented{Feature: "Controller.delContainerNameResolution"}
}

func (c *Controller) addServiceBinding(svcName, svcID, nID, eID, containerName string, vip net.IP, ingressPorts []*PortConfig, serviceAliases, taskAliases []string, ip net.IP, method string) error {
	return errdefs.PlatformNotImplemented{Feature: "Controller.addServiceBinding"}
}

func (c *Controller) rmServiceBinding(svcName, svcID, nID, eID, containerName string, vip net.IP, ingressPorts []*PortConfig, serviceAliases []string, taskAliases []string, ip net.IP, method string, deleteSvcRecords bool, fullRemove bool) error {
	return errdefs.PlatformNotImplemented{Feature: "Controller.rmServiceBinding"}
}

func (sb *Sandbox) populateLoadBalancers(*Endpoint) {}

func arrangeIngressFilterRule() {}
