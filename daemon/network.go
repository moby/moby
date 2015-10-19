package daemon

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/daemon/network"
	"github.com/docker/libnetwork"
)

const (
	// NetworkByID represents a constant to find a network by its ID
	NetworkByID = iota + 1
	// NetworkByName represents a constant to find a network by its Name
	NetworkByName
)

// NetworkControllerEnabled checks if the networking stack is enabled.
// This feature depends on OS primitives and it's dissabled in systems like Windows.
func (daemon *Daemon) NetworkControllerEnabled() bool {
	return daemon.netController != nil
}

// FindNetwork function finds a network for a given string that can represent network name or id
func (daemon *Daemon) FindNetwork(idName string) (libnetwork.Network, error) {
	// Find by Name
	n, err := daemon.GetNetwork(idName, NetworkByName)
	if _, ok := err.(libnetwork.ErrNoSuchNetwork); err != nil && !ok {
		return nil, err
	}

	if n != nil {
		return n, nil
	}

	// Find by id
	n, err = daemon.GetNetwork(idName, NetworkByID)
	if err != nil {
		return nil, err
	}

	return n, nil
}

// GetNetwork function returns a network for a given string that represents the network and
// a hint to indicate if the string is an Id or Name of the network
func (daemon *Daemon) GetNetwork(idName string, by int) (libnetwork.Network, error) {
	c := daemon.netController
	switch by {
	case NetworkByID:
		list := daemon.GetNetworksByID(idName)

		if len(list) == 0 {
			return nil, libnetwork.ErrNoSuchNetwork(idName)
		}

		if len(list) > 1 {
			return nil, libnetwork.ErrInvalidID(idName)
		}

		return list[0], nil
	case NetworkByName:
		if idName == "" {
			idName = c.Config().Daemon.DefaultNetwork
		}
		return c.NetworkByName(idName)
	}
	return nil, errors.New("unexpected selector for GetNetwork")
}

// GetNetworksByID returns a list of networks whose ID partially matches zero or more networks
func (daemon *Daemon) GetNetworksByID(partialID string) []libnetwork.Network {
	c := daemon.netController
	list := []libnetwork.Network{}
	l := func(nw libnetwork.Network) bool {
		if strings.HasPrefix(nw.ID(), partialID) {
			list = append(list, nw)
		}
		return false
	}
	c.WalkNetworks(l)

	return list
}

// CreateNetwork creates a network with the given name, driver and other optional parameters
func (daemon *Daemon) CreateNetwork(name, driver string, ipam network.IPAM, options map[string]string) (libnetwork.Network, error) {
	c := daemon.netController
	if driver == "" {
		driver = c.Config().Daemon.DefaultDriver
	}

	nwOptions := []libnetwork.NetworkOption{}

	v4Conf, v6Conf, err := getIpamConfig(ipam.Config)
	if err != nil {
		return nil, err
	}

	nwOptions = append(nwOptions, libnetwork.NetworkOptionIpam(ipam.Driver, "", v4Conf, v6Conf))
	nwOptions = append(nwOptions, libnetwork.NetworkOptionDriverOpts(options))
	return c.NewNetwork(driver, name, nwOptions...)
}

func getIpamConfig(data []network.IPAMConfig) ([]*libnetwork.IpamConf, []*libnetwork.IpamConf, error) {
	ipamV4Cfg := []*libnetwork.IpamConf{}
	ipamV6Cfg := []*libnetwork.IpamConf{}
	for _, d := range data {
		iCfg := libnetwork.IpamConf{}
		iCfg.PreferredPool = d.Subnet
		iCfg.SubPool = d.IPRange
		iCfg.Gateway = d.Gateway
		iCfg.AuxAddresses = d.AuxAddress
		ip, _, err := net.ParseCIDR(d.Subnet)
		if err != nil {
			return nil, nil, fmt.Errorf("Invalid subnet %s : %v", d.Subnet, err)
		}
		if ip.To4() != nil {
			ipamV4Cfg = append(ipamV4Cfg, &iCfg)
		} else {
			ipamV6Cfg = append(ipamV6Cfg, &iCfg)
		}
	}
	return ipamV4Cfg, ipamV6Cfg, nil
}
