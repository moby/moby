package daemon

import (
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	apierrors "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	networktypes "github.com/docker/libnetwork/types"
	"github.com/pkg/errors"
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
// If no network name is given, the default network is returned.
func (daemon *Daemon) GetNetworkByName(name string) (libnetwork.Network, error) {
	c := daemon.netController
	if c == nil {
		return nil, libnetwork.ErrNoSuchNetwork(name)
	}
	if name == "" {
		name = c.Config().Daemon.DefaultNetwork
	}
	return c.NetworkByName(name)
}

// GetNetworksByID returns a list of networks whose ID partially matches zero or more networks
func (daemon *Daemon) GetNetworksByID(partialID string) []libnetwork.Network {
	c := daemon.netController
	if c == nil {
		return nil
	}
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

// getAllNetworks returns a list containing all networks
func (daemon *Daemon) getAllNetworks() []libnetwork.Network {
	c := daemon.netController
	list := []libnetwork.Network{}
	l := func(nw libnetwork.Network) bool {
		list = append(list, nw)
		return false
	}
	c.WalkNetworks(l)

	return list
}

func isIngressNetwork(name string) bool {
	return name == "ingress"
}

var ingressChan = make(chan struct{}, 1)

func ingressWait() func() {
	ingressChan <- struct{}{}
	return func() { <-ingressChan }
}

// SetNetworkBootstrapKeys sets the bootstrap keys.
func (daemon *Daemon) SetNetworkBootstrapKeys(keys []*networktypes.EncryptionKey) error {
	return daemon.netController.SetKeys(keys)
}

// CreateNetwork creates a network with the given name, driver and other optional parameters
func (daemon *Daemon) CreateNetwork(create types.NetworkCreateRequest) (*types.NetworkCreateResponse, error) {
	resp, err := daemon.createNetwork(create, "", false)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (daemon *Daemon) createNetwork(create types.NetworkCreateRequest, id string, agent bool) (*types.NetworkCreateResponse, error) {
	if runconfig.IsPreDefinedNetwork(create.Name) && !agent {
		err := fmt.Errorf("%s is a pre-defined network and cannot be created", create.Name)
		return nil, apierrors.NewRequestForbiddenError(err)
	}

	var warning string
	nw, err := daemon.GetNetworkByName(create.Name)
	if err != nil {
		if _, ok := err.(libnetwork.ErrNoSuchNetwork); !ok {
			return nil, err
		}
	}
	if nw != nil {
		if create.CheckDuplicate {
			return nil, libnetwork.NetworkNameError(create.Name)
		}
		warning = fmt.Sprintf("Network with name %s (id : %s) already exists", nw.Name(), nw.ID())
	}

	c := daemon.netController
	driver := create.Driver
	if driver == "" {
		driver = c.Config().Daemon.DefaultDriver
	}

	nwOptions := []libnetwork.NetworkOption{
		libnetwork.NetworkOptionEnableIPv6(create.EnableIPv6),
		libnetwork.NetworkOptionDriverOpts(create.Options),
		libnetwork.NetworkOptionLabels(create.Labels),
		libnetwork.NetworkOptionAttachable(create.Attachable),
	}

	if create.IPAM != nil {
		ipam := create.IPAM
		v4Conf, v6Conf, err := getIpamConfig(ipam.Config)
		if err != nil {
			return nil, err
		}
		nwOptions = append(nwOptions, libnetwork.NetworkOptionIpam(ipam.Driver, "", v4Conf, v6Conf, ipam.Options))
	}

	if create.Internal {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionInternalNetwork())
	}
	if agent {
		nwOptions = append(nwOptions, libnetwork.NetworkOptionDynamic())
		nwOptions = append(nwOptions, libnetwork.NetworkOptionPersist(false))
	}

	n, err := c.NewNetwork(driver, create.Name, id, nwOptions...)
	if err != nil {
		return nil, err
	}

	daemon.pluginRefCount(driver, driverapi.NetworkPluginEndpointType, plugingetter.ACQUIRE)
	if create.IPAM != nil {
		daemon.pluginRefCount(create.IPAM.Driver, ipamapi.PluginEndpointType, plugingetter.ACQUIRE)
	}
	daemon.LogNetworkEvent(n, "create")

	return &types.NetworkCreateResponse{
		ID:      n.ID(),
		Warning: warning,
	}, nil
}

func (daemon *Daemon) pluginRefCount(driver, capability string, mode int) {
	var builtinDrivers []string

	if capability == driverapi.NetworkPluginEndpointType {
		builtinDrivers = daemon.netController.BuiltinDrivers()
	} else if capability == ipamapi.PluginEndpointType {
		builtinDrivers = daemon.netController.BuiltinIPAMDrivers()
	}

	for _, d := range builtinDrivers {
		if d == driver {
			return
		}
	}

	if daemon.PluginStore != nil {
		_, err := daemon.PluginStore.Get(driver, capability, mode)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"mode": mode, "driver": driver}).Error("Error handling plugin refcount operation")
		}
	}
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
	if runtime.GOOS == "solaris" {
		return errors.New("docker network connect is unsupported on Solaris platform")
	}
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return err
	}
	return daemon.ConnectToNetwork(container, networkName, endpointConfig)
}

// DisconnectContainerFromNetwork disconnects the given container from
// the given network. If either cannot be found, an err is returned.
func (daemon *Daemon) DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error {
	if runtime.GOOS == "solaris" {
		return errors.New("docker network disconnect is unsupported on Solaris platform")
	}
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		if force {
			return daemon.ForceEndpointDelete(containerName, networkName)
		}
		return err
	}
	return daemon.DisconnectFromNetwork(container, networkName, force)
}

// GetNetworkDriverList returns the list of plugins drivers
// registered for network.
func (daemon *Daemon) GetNetworkDriverList() []string {
	if !daemon.NetworkControllerEnabled() {
		return nil
	}

	pluginList := daemon.netController.BuiltinDrivers()

	managedPlugins := daemon.PluginStore.GetAllManagedPluginsByCap(driverapi.NetworkPluginEndpointType)

	for _, plugin := range managedPlugins {
		pluginList = append(pluginList, plugin.Name())
	}

	pluginMap := make(map[string]bool)
	for _, plugin := range pluginList {
		pluginMap[plugin] = true
	}

	networks := daemon.netController.Networks()

	for _, network := range networks {
		if !pluginMap[network.Type()] {
			pluginList = append(pluginList, network.Type())
			pluginMap[network.Type()] = true
		}
	}

	sort.Strings(pluginList)

	return pluginList
}

// DeleteManagedNetwork deletes an agent network.
func (daemon *Daemon) DeleteManagedNetwork(networkID string) error {
	return daemon.deleteNetwork(networkID, true)
}

// DeleteNetwork destroys a network unless it's one of docker's predefined networks.
func (daemon *Daemon) DeleteNetwork(networkID string) error {
	return daemon.deleteNetwork(networkID, false)
}

func (daemon *Daemon) deleteNetwork(networkID string, dynamic bool) error {
	nw, err := daemon.FindNetwork(networkID)
	if err != nil {
		return err
	}

	if runconfig.IsPreDefinedNetwork(nw.Name()) && !dynamic {
		err := fmt.Errorf("%s is a pre-defined network and cannot be removed", nw.Name())
		return apierrors.NewRequestForbiddenError(err)
	}

	if err := nw.Delete(); err != nil {
		return err
	}
	daemon.pluginRefCount(nw.Type(), driverapi.NetworkPluginEndpointType, plugingetter.RELEASE)
	ipamType, _, _, _ := nw.Info().IpamConfig()
	daemon.pluginRefCount(ipamType, ipamapi.PluginEndpointType, plugingetter.RELEASE)
	daemon.LogNetworkEvent(nw, "destroy")
	return nil
}

// GetNetworks returns a list of all networks
func (daemon *Daemon) GetNetworks() []libnetwork.Network {
	return daemon.getAllNetworks()
}

// clearAttachableNetworks removes the attachable networks
// after disconnecting any connected container
func (daemon *Daemon) clearAttachableNetworks() {
	for _, n := range daemon.GetNetworks() {
		if !n.Info().Attachable() {
			continue
		}
		for _, ep := range n.Endpoints() {
			epInfo := ep.Info()
			if epInfo == nil {
				continue
			}
			sb := epInfo.Sandbox()
			if sb == nil {
				continue
			}
			containerID := sb.ContainerID()
			if err := daemon.DisconnectContainerFromNetwork(containerID, n.ID(), true); err != nil {
				logrus.Warnf("Failed to disconnect container %s from swarm network %s on cluster leave: %v",
					containerID, n.Name(), err)
			}
		}
		if err := daemon.DeleteManagedNetwork(n.ID()); err != nil {
			logrus.Warnf("Failed to remove swarm network %s on cluster leave: %v", n.Name(), err)
		}
	}
}
