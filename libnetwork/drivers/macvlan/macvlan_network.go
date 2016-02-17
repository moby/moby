package macvlan

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
)

// CreateNetwork the network for the specified driver type
func (d *driver) CreateNetwork(id string, option map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	// parse and validate the config and bind to networkConfiguration
	config, err := parseNetworkOptions(id, option)
	if err != nil {
		return err
	}
	config.ID = id
	err = config.processIPAM(id, ipV4Data, ipV6Data)
	if err != nil {
		return err
	}
	// user must specify a parent interface on the host to attach the container vif -o host_iface=eth0
	if config.HostIface == "" {
		return fmt.Errorf("%s requires an interface from the docker host to be specified (usage: -o host_iface=eth)", macvlanType)
	}
	// loopback is not a valid parent link
	if config.HostIface == "lo" {
		return fmt.Errorf("loopback interface is not a valid %s parent link", macvlanType)
	}
	// verify the macvlan mode from -o macvlan_mode option
	switch config.MacvlanMode {
	case "", modeBridge:
		// default to macvlan bridge mode if -o macvlan_mode is empty
		config.MacvlanMode = modeBridge
	case modeOpt:
		config.MacvlanMode = modeOpt
	case modePassthru:
		config.MacvlanMode = modePassthru
	case modeVepa:
		config.MacvlanMode = modeVepa
	default:
		return fmt.Errorf("requested macvlan mode '%s' is not valid, 'bridge' mode is the macvlan driver default", config.MacvlanMode)
	}
	err = d.createNetwork(config)
	if err != nil {
		return err
	}
	// update persistent db, rollback on fail
	err = d.storeUpdate(config)
	if err != nil {
		d.deleteNetwork(config.ID)
		return err
	}

	return nil
}

// createNetwork is used by new network callbacks and persistent network cache
func (d *driver) createNetwork(config *configuration) error {
	// if the -o host_iface does not exist, attempt to parse a parent_iface.vlan_id
	networkList := d.getNetworks()
	for _, nw := range networkList {
		if config.HostIface == nw.config.HostIface {
			return fmt.Errorf("network %s is already using host interface %s",
				stringid.TruncateID(nw.config.ID), config.HostIface)
		}
	}
	// if the -o host_iface does not exist, attempt to parse a parent_iface.vlan_id
	if ok := hostIfaceExists(config.HostIface); !ok {
		// if the subinterface parent_iface.vlan_id checks do not pass, return err.
		//  a valid example is eth0.10 for a parent iface: eth0 with a vlan id: 10
		err := createVlanLink(config.HostIface)
		if err != nil {
			return err
		}
		// if driver created the networks slave link, record it for future deletion
		config.CreatedSlaveLink = true
	}
	n := &network{
		id:        config.ID,
		driver:    d,
		endpoints: endpointTable{},
		config:    config,
	}
	// add the *network
	d.addNetwork(n)

	return nil
}

// DeleteNetwork the network for the specified driver type
func (d *driver) DeleteNetwork(nid string) error {
	n := d.network(nid)
	// if the driver created the slave interface, delete it, otherwise leave it
	if ok := n.config.CreatedSlaveLink; ok {
		// if the interface exists, only delete if it matches iface.vlan naming
		if ok := hostIfaceExists(n.config.HostIface); ok {
			err := delVlanLink(n.config.HostIface)
			if err != nil {
				logrus.Debugf("link %s was not deleted, continuing the delete network operation: %v", n.config.HostIface, err)
			}
		}
	}
	// delete the *network
	d.deleteNetwork(nid)
	// delete the network record from persistent cache
	return d.storeDelete(n.config)
}

// parseNetworkOptions parse docker network options
func parseNetworkOptions(id string, option options.Generic) (*configuration, error) {
	var (
		err    error
		config = &configuration{}
	)
	// parse generic labels first
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		if config, err = parseNetworkGenericOptions(genData); err != nil {
			return nil, err
		}
	}
	// return an error if the unsupported --internal is passed
	if _, ok := option[netlabel.Internal]; ok {
		return nil, fmt.Errorf("--internal option is not supported with the macvlan driver")
	}
	return config, nil
}

// parseNetworkGenericOptions parse generic driver docker network options
func parseNetworkGenericOptions(data interface{}) (*configuration, error) {
	var (
		err    error
		config *configuration
	)
	switch opt := data.(type) {
	case *configuration:
		config = opt
	case map[string]string:
		config = &configuration{}
		err = config.fromOptions(opt)
	case options.Generic:
		var opaqueConfig interface{}
		if opaqueConfig, err = options.GenerateFromModel(opt, config); err == nil {
			config = opaqueConfig.(*configuration)
		}
	default:
		err = types.BadRequestErrorf("unrecognized network configuration format: %v", opt)
	}
	return config, err
}

// fromOptions binds the generic options to networkConfiguration to cache
func (config *configuration) fromOptions(labels map[string]string) error {
	for label, value := range labels {
		switch label {
		case hostIfaceOpt:
			// parse driver option '-o host_iface'
			config.HostIface = value
		case driverModeOpt:
			// parse driver option '-o macvlan_mode'
			config.MacvlanMode = value
		}
	}
	return nil
}

// processIPAM parses v4 and v6 IP information and binds it to the network configuration
func (config *configuration) processIPAM(id string, ipamV4Data, ipamV6Data []driverapi.IPAMData) error {
	if len(ipamV4Data) > 0 {
		for _, ipd := range ipamV4Data {
			s := &ipv4Subnet{
				SubnetIP: ipd.Pool.String(),
				GwIP:     ipd.Gateway.String(),
			}
			config.Ipv4Subnets = append(config.Ipv4Subnets, s)
		}
	}
	if len(ipamV6Data) > 0 {
		for _, ipd := range ipamV6Data {
			s := &ipv6Subnet{
				SubnetIP: ipd.Pool.String(),
				GwIP:     ipd.Gateway.String(),
			}
			config.Ipv6Subnets = append(config.Ipv6Subnets, s)
		}
	}
	return nil
}
