package network

import (
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
)

var (
	ErrNoDefaultRoute = errors.New("no default network route found")
)

func InterfaceUp(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkLinkUp(iface)
}

func InterfaceDown(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkLinkDown(iface)
}

func ChangeInterfaceName(old, newName string) error {
	iface, err := net.InterfaceByName(old)
	if err != nil {
		return err
	}
	return netlink.NetworkChangeName(iface, newName)
}

func CreateVethPair(name1, name2 string) error {
	return netlink.NetworkCreateVethPair(name1, name2)
}

func SetInterfaceInNamespacePid(name string, nsPid int) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkSetNsPid(iface, nsPid)
}

func SetInterfaceInNamespaceFd(name string, fd int) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkSetNsFd(iface, fd)
}

func SetInterfaceMaster(name, master string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	masterIface, err := net.InterfaceByName(master)
	if err != nil {
		return err
	}
	return netlink.NetworkSetMaster(iface, masterIface)
}

func SetDefaultGateway(ip string) error {
	return netlink.AddDefaultGw(net.ParseIP(ip))
}

func SetInterfaceIp(name string, rawIp string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	ip, ipNet, err := net.ParseCIDR(rawIp)
	if err != nil {
		return err
	}
	return netlink.NetworkLinkAddIp(iface, ip, ipNet)
}

func SetMtu(name string, mtu int) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkSetMTU(iface, mtu)
}

func GetDefaultMtu() (int, error) {
	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		return -1, err
	}
	for _, r := range routes {
		if r.Default {
			return r.Iface.MTU, nil
		}
	}
	return -1, ErrNoDefaultRoute
}
