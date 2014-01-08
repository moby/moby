package netlink

import (
	"fmt"
	"net"
)

func NetworkGetRoutes() ([]*net.IPNet, error) {
	return nil, fmt.Errorf("Not implemented")
}

func NetworkLinkAdd(name string, linkType string) error {
	return fmt.Errorf("Not implemented")
}

func NetworkLinkUp(iface *net.Interface) error {
	return fmt.Errorf("Not implemented")
}

func NetworkLinkAddIp(iface *net.Interface, ip net.IP, ipNet *net.IPNet) error {
	return fmt.Errorf("Not implemented")
}

func AddDefaultGw(ip net.IP) error {
	return fmt.Errorf("Not implemented")

}

func NetworkSetMTU(iface *net.Interface, mtu int) error {
	return fmt.Errorf("Not implemented")
}
