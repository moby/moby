package daemon

import (
	"errors"
	"strings"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
)

const (
	// NetworkByID represents a constant to find a network by its ID
	NetworkByID = iota + 1
	// NetworkByName represents a constant to find a network by its Name
	NetworkByName
)

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
func (daemon *Daemon) CreateNetwork(name, driver string, options map[string]interface{}) (libnetwork.Network, error) {
	c := daemon.netController
	if driver == "" {
		driver = c.Config().Daemon.DefaultDriver
	}

	if options == nil {
		options = make(map[string]interface{})
	}
	_, ok := options[netlabel.GenericData]
	if !ok {
		options[netlabel.GenericData] = make(map[string]interface{})
	}

	return c.NewNetwork(driver, name, parseOptions(options)...)
}

func parseOptions(options map[string]interface{}) []libnetwork.NetworkOption {
	var setFctList []libnetwork.NetworkOption

	if options != nil {
		setFctList = append(setFctList, libnetwork.NetworkOptionGeneric(options))
	}

	return setFctList
}
