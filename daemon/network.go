package daemon

import (
	"fmt"
	"net"
	"strings"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/runconfig"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/libnetwork"
)

// NetworkControllerEnabled checks if the networking stack is enabled.
// This feature depends on OS primitives and it's disabled in systems like Windows.
func (daemon *Daemon) NetworkControllerEnabled() bool {
	return daemon.netController != nil
}

// FindNetwork function finds a network for a given string that can represent network name or id
func (daemon *Daemon) FindNetwork(idName string) (libnetwork.Network, error) {
	// Find by Name
	n, err := daemon.GetNetworkByName(idName)
	if err != nil && !isNoSuchNetworkError(err) {
		return nil, err
	}

	if n != nil {
		return n, nil
	}

	// Find by id
	return daemon.GetNetworkByID(idName)
}

func isNoSuchNetworkError(err error) bool {
	_, ok := err.(libnetwork.ErrNoSuchNetwork)
	return ok
}

// GetNetworkByID function returns a network whose ID begins with the given prefix.
// It fails with an error if no matching, or more than one matching, networks are found.
func (daemon *Daemon) GetNetworkByID(partialID string) (libnetwork.Network, error) {
	list := daemon.GetNetworksByID(partialID)

	if len(list) == 0 {
		return nil, libnetwork.ErrNoSuchNetwork(partialID)
	}
	if len(list) > 1 {
		return nil, libnetwork.ErrInvalidID(partialID)
	}
	return list[0], nil
}

// GetNetworkByName function returns a network for a given network name.
func (daemon *Daemon) GetNetworkByName(name string) (libnetwork.Network, error) {
	c := daemon.netController
	if name == "" {
		name = c.Config().Daemon.DefaultNetwork
	}
	return c.NetworkByName(name)
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

// GetAllNetworks returns a list containing all networks
func (daemon *Daemon) GetAllNetworks() []libnetwork.Network {
	c := daemon.netController
	list := []libnetwork.Network{}
	l := func(nw libnetwork.Network) bool {
		list = append(list, nw)
		return false
	}
	c.WalkNetworks(l)

	return list
}

// CreateNetwork creates a network with the given name, driver and other optional parameters
func (daemon *Daemon) CreateNetwork(name, driver string, ipam network.IPAM, options map[string]string, internal bool) (libnetwork.Network, error) {
	c := daemon.netController
	if driver == "" {
		driver = c.Config().Daemon.DefaultDriver
	}

	nwOptions := []libnetwork.NetworkOption{}

	v4Conf, v6Conf, err := getIpamConfig(ipam.Config)
	if err != nil {
		return nil, err
	}

	nwOptions = append(nwOptions, libnetwork.NetworkOptionIpam(ipam.Driver, "", v4Conf, v6Conf, ipam.Options))
	nwOptions = append(nwOptions, libnetwork.NetworkOptionDriverOpts(options))
	if internal {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionInternalNetwork())
	}
	n, err := c.NewNetwork(driver, name, nwOptions...)
	if err != nil {
		return nil, err
	}

	daemon.LogNetworkEvent(n, "create")
	return n, nil
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

// ConnectContainerToNetwork connects the given container to the given
// network. If either cannot be found, an err is returned. If the
// network cannot be set up, an err is returned.
func (daemon *Daemon) ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error {
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}
	return daemon.ConnectToNetwork(container, networkName, endpointConfig)
}

// DisconnectContainerFromNetwork disconnects the given container from
// the given network. If either cannot be found, an err is returned.
func (daemon *Daemon) DisconnectContainerFromNetwork(containerName string, network libnetwork.Network, force bool) error {
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		if force {
			return daemon.ForceEndpointDelete(containerName, network)
		}
		return err
	}
	return daemon.DisconnectFromNetwork(container, network, force)
}

// GetNetworkDriverList returns the list of plugins drivers
// registered for network.
func (daemon *Daemon) GetNetworkDriverList() map[string]bool {
	pluginList := make(map[string]bool)

	if !daemon.NetworkControllerEnabled() {
		return nil
	}
	c := daemon.netController
	networks := c.Networks()

	for _, network := range networks {
		driver := network.Type()
		pluginList[driver] = true
	}

	return pluginList
}

// DeleteNetwork destroys a network unless it's one of docker's predefined networks.
func (daemon *Daemon) DeleteNetwork(networkID string) error {
	nw, err := daemon.FindNetwork(networkID)
	if err != nil {
		return err
	}

	if runconfig.IsPreDefinedNetwork(nw.Name()) {
		return derr.ErrorCodeCantDeletePredefinedNetwork.WithArgs(nw.Name())
	}

	if err := nw.Delete(); err != nil {
		return err
	}
	daemon.LogNetworkEvent(nw, "destroy")
	return nil
}
