//go:build linux

package macvlan

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/daemon/libnetwork/options"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/errdefs"
)

// CreateNetwork the network for the specified driver type
func (d *driver) CreateNetwork(ctx context.Context, nid string, option map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	// reject a null v4 network if ipv4 is required
	if v, ok := option[netlabel.EnableIPv4]; ok && v.(bool) {
		if len(ipV4Data) == 0 || ipV4Data[0].Pool.String() == "0.0.0.0/0" {
			return errdefs.InvalidParameter(errors.New("ipv4 pool is empty"))
		}
	}
	// reject a null v6 network if ipv6 is required
	if v, ok := option[netlabel.EnableIPv6]; ok && v.(bool) {
		if len(ipV6Data) == 0 || ipV6Data[0].Pool.String() == "::/0" {
			return errdefs.InvalidParameter(errors.New("ipv6 pool is empty"))
		}
	}

	// parse and validate the config and bind to networkConfiguration
	config, err := parseNetworkOptions(nid, option)
	if err != nil {
		return err
	}
	config.processIPAM(ipV4Data, ipV6Data)

	// if parent interface not specified, create a dummy type link to use named dummy+net_id
	if config.Parent == "" {
		config.Parent = getDummyName(config.ID)
	}
	foundExisting, err := d.createNetwork(config)
	if err != nil {
		return err
	}

	if foundExisting {
		return types.InternalMaskableErrorf("restoring existing network %s", config.ID)
	}

	// update persistent db, rollback on fail
	err = d.storeUpdate(config)
	if err != nil {
		d.deleteNetwork(config.ID)
		log.G(ctx).Debugf("encountered an error rolling back a network create for %s : %v", config.ID, err)
		return err
	}

	return nil
}

func (d *driver) GetSkipGwAlloc(options.Generic) (ipv4, ipv6 bool, _ error) {
	// Only set up a default gateway if the user configured one (the gateway
	// must be external to the Docker macvlan network, the driver doesn't assign
	// the address to anything).
	return true, true, nil
}

// createNetwork is used by new network callbacks and persistent network cache
func (d *driver) createNetwork(config *configuration) (bool, error) {
	foundExisting := false
	networkList := d.getNetworks()
	for _, nw := range networkList {
		if config.Parent == nw.config.Parent {
			if config.ID != nw.config.ID {
				if config.MacvlanMode == modePassthru {
					return false, fmt.Errorf(
						"cannot use mode passthru, macvlan network %s is already using parent interface %s",
						nw.config.ID,
						config.Parent,
					)
				} else if nw.config.MacvlanMode == modePassthru {
					return false, fmt.Errorf(
						"macvlan network %s is already using parent interface %s in mode passthru",
						nw.config.ID,
						config.Parent,
					)
				}
				continue
			}
			log.G(context.TODO()).Debugf("Create Network for the same ID %s\n", config.ID)
			foundExisting = true
			break
		}
	}
	if !parentExists(config.Parent) {
		// Create a dummy link if a dummy name is set for parent
		if dummyName := getDummyName(config.ID); dummyName == config.Parent {
			err := createDummyLink(config.Parent, dummyName)
			if err != nil {
				return false, err
			}
			config.CreatedSlaveLink = true

			// notify the user in logs that they have limited communications
			log.G(context.TODO()).Debugf("Empty -o parent= flags limit communications to other containers inside of network: %s",
				config.Parent)
		} else {
			// if the subinterface parent_iface.vlan_id checks do not pass, return err.
			//  a valid example is 'eth0.10' for a parent iface 'eth0' with a vlan id '10'
			err := createVlanLink(config.Parent)
			if err != nil {
				return false, err
			}
			// if driver created the networks slave link, record it for future deletion
			config.CreatedSlaveLink = true
		}
	} else {
		// Check and mark this network if the interface was created for another network
		for _, testN := range d.getNetworks() {
			if config.Parent == testN.config.Parent && testN.config.CreatedSlaveLink {
				config.CreatedSlaveLink = true
				break
			}
		}
	}
	if !foundExisting {
		d.addNetwork(&network{
			id:        config.ID,
			driver:    d,
			endpoints: map[string]*endpoint{},
			config:    config,
		})
	}

	return foundExisting, nil
}

func (d *driver) parentHasSingleUser(parent string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	users := 0
	for _, nw := range d.networks {
		if nw.config.Parent == parent {
			users++
		}
		if users > 1 {
			return false
		}
	}
	// TODO(thaJeztah): "zero users" should also return "true?" (this would be theoretical as we're checking a network to be the last remaining user)
	return users == 1
}

// DeleteNetwork deletes the network for the specified driver type
func (d *driver) DeleteNetwork(nid string) error {
	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %s not found", nid)
	}
	// if the driver created the slave interface and this network is the last user, delete it, otherwise leave it
	if n.config.CreatedSlaveLink && parentExists(n.config.Parent) && d.parentHasSingleUser(n.config.Parent) {
		// only delete the link if it is named the net_id
		if n.config.Parent == getDummyName(nid) {
			if err := delDummyLink(n.config.Parent); err != nil {
				log.G(context.TODO()).WithError(err).Debugf("link %s was not deleted, continuing the delete network operation", n.config.Parent)
			}
		} else {
			// only delete the link if it matches iface.vlan naming
			if err := delVlanLink(n.config.Parent); err != nil {
				log.G(context.TODO()).WithError(err).Debugf("link %s was not deleted, continuing the delete network operation", n.config.Parent)
			}
		}
	}
	for _, ep := range n.endpoints {
		if link, err := ns.NlHandle().LinkByName(ep.srcName); err == nil {
			if err := ns.NlHandle().LinkDel(link); err != nil {
				log.G(context.TODO()).WithError(err).Warnf("Failed to delete interface (%s)'s link on endpoint (%s) delete", ep.srcName, ep.id)
			}
		}

		if err := d.storeDelete(ep); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to remove macvlan endpoint %.7s from store", ep.id)
		}
	}
	// delete the *network
	d.deleteNetwork(nid)
	// delete the network record from persistent cache
	if err := d.storeDelete(n.config); err != nil {
		return fmt.Errorf("error deleting id %s from datastore: %v", nid, err)
	}
	return nil
}

// parseNetworkOptions parses docker network options
func parseNetworkOptions(id string, option options.Generic) (*configuration, error) {
	config := &configuration{}

	// parse generic labels first
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		var err error
		config, err = parseNetworkGenericOptions(genData)
		if err != nil {
			return nil, err
		}
	}
	if val, ok := option[netlabel.Internal]; ok {
		if internal, ok := val.(bool); ok && internal {
			config.Internal = true
		}
	}

	// verify the macvlan mode from -o macvlan_mode option
	switch config.MacvlanMode {
	case "":
		// default to macvlan bridge mode if -o macvlan_mode is empty
		config.MacvlanMode = modeBridge
	case modeBridge, modePrivate, modePassthru, modeVepa:
		// valid option
	default:
		return nil, fmt.Errorf("requested macvlan mode '%s' is not valid, 'bridge' mode is the macvlan driver default", config.MacvlanMode)
	}

	// loopback is not a valid parent link
	if config.Parent == "lo" {
		return nil, errors.New("loopback interface is not a valid macvlan parent link")
	}

	// With no parent interface, the network is "internal".
	if config.Parent == "" {
		config.Internal = true
	}

	config.ID = id
	return config, nil
}

// parseNetworkGenericOptions parses generic driver docker network options
func parseNetworkGenericOptions(data any) (*configuration, error) {
	switch opt := data.(type) {
	case *configuration:
		return opt, nil
	case map[string]string:
		return newConfigFromLabels(opt), nil
	default:
		return nil, types.InvalidParameterErrorf("unrecognized network configuration format: %v", opt)
	}
}

// newConfigFromLabels creates a new configuration from the given labels.
func newConfigFromLabels(labels map[string]string) *configuration {
	config := &configuration{}
	for label, value := range labels {
		switch label {
		case parentOpt:
			// parse driver option '-o parent'
			config.Parent = value
		case driverModeOpt:
			// parse driver option '-o macvlan_mode'
			config.MacvlanMode = value
		}
	}

	return config
}

// processIPAM parses v4 and v6 IP information and binds it to the network configuration
func (config *configuration) processIPAM(ipamV4Data, ipamV6Data []driverapi.IPAMData) {
	for _, ipd := range ipamV4Data {
		s := &ipSubnet{SubnetIP: ipd.Pool.String()}
		if ipd.Gateway != nil {
			s.GwIP = ipd.Gateway.String()
		}
		config.Ipv4Subnets = append(config.Ipv4Subnets, s)
	}
	for _, ipd := range ipamV6Data {
		s := &ipSubnet{SubnetIP: ipd.Pool.String()}
		if ipd.Gateway != nil {
			s.GwIP = ipd.Gateway.String()
		}
		config.Ipv6Subnets = append(config.Ipv6Subnets, s)
	}
}
