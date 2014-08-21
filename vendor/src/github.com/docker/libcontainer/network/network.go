// +build linux

package network

import (
	"net"

	"github.com/docker/libcontainer/netlink"
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

func SetInterfaceInNamespaceFd(name string, fd uintptr) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkSetNsFd(iface, int(fd))
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
	return netlink.AddToBridge(iface, masterIface)
}

func SetDefaultGateway(ip, ifaceName string) error {
	return netlink.AddDefaultGw(ip, ifaceName)
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
