//go:build linux

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
)

const (
	// network config prefix was not specific enough.
	// To be backward compatible, need custom endpoint
	// prefix with different root
	bridgePrefix         = "bridge"
	bridgeEndpointPrefix = "bridge-endpoint"
)

func (d *driver) initStore(option map[string]interface{}) error {
	if data, ok := option[netlabel.LocalKVClient]; ok {
		var ok bool
		d.store, ok = data.(*datastore.Store)
		if !ok {
			return types.InternalErrorf("incorrect data in datastore configuration: %v", data)
		}

		err := d.populateNetworks()
		if err != nil {
			return err
		}

		err = d.populateEndpoints()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *driver) populateNetworks() error {
	kvol, err := d.store.List(&networkConfiguration{})
	if err != nil && err != datastore.ErrKeyNotFound {
		return fmt.Errorf("failed to get bridge network configurations from store: %v", err)
	}

	// It's normal for network configuration state to be empty. Just return.
	if err == datastore.ErrKeyNotFound {
		return nil
	}

	for _, kvo := range kvol {
		ncfg := kvo.(*networkConfiguration)
		if err = d.createNetwork(ncfg); err != nil {
			log.G(context.TODO()).Warnf("could not create bridge network for id %s bridge name %s while booting up from persistent state: %v", ncfg.ID, ncfg.BridgeName, err)
		}
		log.G(context.TODO()).Debugf("Network (%.7s) restored", ncfg.ID)
	}

	return nil
}

func (d *driver) populateEndpoints() error {
	kvol, err := d.store.List(&bridgeEndpoint{})
	if err != nil && err != datastore.ErrKeyNotFound {
		return fmt.Errorf("failed to get bridge endpoints from store: %v", err)
	}

	if err == datastore.ErrKeyNotFound {
		return nil
	}

	for _, kvo := range kvol {
		ep := kvo.(*bridgeEndpoint)
		n, ok := d.networks[ep.nid]
		if !ok {
			log.G(context.TODO()).Debugf("Network (%.7s) not found for restored bridge endpoint (%.7s)", ep.nid, ep.id)
			log.G(context.TODO()).Debugf("Deleting stale bridge endpoint (%.7s) from store", ep.id)
			if err := d.storeDelete(ep); err != nil {
				log.G(context.TODO()).Debugf("Failed to delete stale bridge endpoint (%.7s) from store", ep.id)
			}
			continue
		}
		n.endpoints[ep.id] = ep
		n.restorePortAllocations(ep)
		log.G(context.TODO()).Debugf("Endpoint (%.7s) restored to network (%.7s)", ep.id, ep.nid)
	}

	return nil
}

func (d *driver) storeUpdate(kvObject datastore.KVObject) error {
	if d.store == nil {
		log.G(context.TODO()).Warnf("bridge store not initialized. kv object %s is not added to the store", datastore.Key(kvObject.Key()...))
		return nil
	}

	if err := d.store.PutObjectAtomic(kvObject); err != nil {
		return fmt.Errorf("failed to update bridge store for object type %T: %v", kvObject, err)
	}

	return nil
}

func (d *driver) storeDelete(kvObject datastore.KVObject) error {
	if d.store == nil {
		log.G(context.TODO()).Debugf("bridge store not initialized. kv object %s is not deleted from store", datastore.Key(kvObject.Key()...))
		return nil
	}

	return d.store.DeleteObject(kvObject)
}

func (ncfg *networkConfiguration) MarshalJSON() ([]byte, error) {
	nMap := make(map[string]interface{})
	nMap["ID"] = ncfg.ID
	nMap["BridgeName"] = ncfg.BridgeName
	nMap["EnableIPv6"] = ncfg.EnableIPv6
	nMap["EnableIPMasquerade"] = ncfg.EnableIPMasquerade
	nMap["GwModeIPv4"] = ncfg.GwModeIPv4
	nMap["GwModeIPv6"] = ncfg.GwModeIPv6
	nMap["EnableICC"] = ncfg.EnableICC
	nMap["InhibitIPv4"] = ncfg.InhibitIPv4
	nMap["Mtu"] = ncfg.Mtu
	nMap["Internal"] = ncfg.Internal
	nMap["DefaultBridge"] = ncfg.DefaultBridge
	nMap["DefaultBindingIP"] = ncfg.DefaultBindingIP.String()
	// This key is "HostIP" instead of "HostIPv4" to preserve compatibility with the on-disk format.
	nMap["HostIP"] = ncfg.HostIPv4.String()
	nMap["HostIPv6"] = ncfg.HostIPv6.String()
	nMap["DefaultGatewayIPv4"] = ncfg.DefaultGatewayIPv4.String()
	nMap["DefaultGatewayIPv6"] = ncfg.DefaultGatewayIPv6.String()
	nMap["ContainerIfacePrefix"] = ncfg.ContainerIfacePrefix
	nMap["BridgeIfaceCreator"] = ncfg.BridgeIfaceCreator

	if ncfg.AddressIPv4 != nil {
		nMap["AddressIPv4"] = ncfg.AddressIPv4.String()
	}

	if ncfg.AddressIPv6 != nil {
		nMap["AddressIPv6"] = ncfg.AddressIPv6.String()
	}

	return json.Marshal(nMap)
}

func (ncfg *networkConfiguration) UnmarshalJSON(b []byte) error {
	var (
		err  error
		nMap map[string]interface{}
	)

	if err = json.Unmarshal(b, &nMap); err != nil {
		return err
	}

	if v, ok := nMap["AddressIPv4"]; ok {
		if ncfg.AddressIPv4, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode bridge network address IPv4 after json unmarshal: %s", v.(string))
		}
	}

	if v, ok := nMap["AddressIPv6"]; ok {
		if ncfg.AddressIPv6, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode bridge network address IPv6 after json unmarshal: %s", v.(string))
		}
	}

	if v, ok := nMap["ContainerIfacePrefix"]; ok {
		ncfg.ContainerIfacePrefix = v.(string)
	}

	// This key is "HostIP" instead of "HostIPv4" to preserve compatibility with the on-disk format.
	if v, ok := nMap["HostIP"]; ok {
		ncfg.HostIPv4 = net.ParseIP(v.(string))
	}
	if v, ok := nMap["HostIPv6"]; ok {
		ncfg.HostIPv6 = net.ParseIP(v.(string))
	}

	ncfg.DefaultBridge = nMap["DefaultBridge"].(bool)
	ncfg.DefaultBindingIP = net.ParseIP(nMap["DefaultBindingIP"].(string))
	ncfg.DefaultGatewayIPv4 = net.ParseIP(nMap["DefaultGatewayIPv4"].(string))
	ncfg.DefaultGatewayIPv6 = net.ParseIP(nMap["DefaultGatewayIPv6"].(string))
	ncfg.ID = nMap["ID"].(string)
	ncfg.BridgeName = nMap["BridgeName"].(string)
	ncfg.EnableIPv6 = nMap["EnableIPv6"].(bool)
	ncfg.EnableIPMasquerade = nMap["EnableIPMasquerade"].(bool)
	if v, ok := nMap["GwModeIPv4"]; ok {
		ncfg.GwModeIPv4, _ = newGwMode(v.(string))
	}
	if v, ok := nMap["GwModeIPv6"]; ok {
		ncfg.GwModeIPv6, _ = newGwMode(v.(string))
	}
	ncfg.EnableICC = nMap["EnableICC"].(bool)
	if v, ok := nMap["InhibitIPv4"]; ok {
		ncfg.InhibitIPv4 = v.(bool)
	}

	ncfg.Mtu = int(nMap["Mtu"].(float64))
	if v, ok := nMap["Internal"]; ok {
		ncfg.Internal = v.(bool)
	}

	if v, ok := nMap["BridgeIfaceCreator"]; ok {
		ncfg.BridgeIfaceCreator = ifaceCreator(v.(float64))
	}

	return nil
}

func (ncfg *networkConfiguration) Key() []string {
	return []string{bridgePrefix, ncfg.ID}
}

func (ncfg *networkConfiguration) KeyPrefix() []string {
	return []string{bridgePrefix}
}

func (ncfg *networkConfiguration) Value() []byte {
	b, err := json.Marshal(ncfg)
	if err != nil {
		return nil
	}
	return b
}

func (ncfg *networkConfiguration) SetValue(value []byte) error {
	return json.Unmarshal(value, ncfg)
}

func (ncfg *networkConfiguration) Index() uint64 {
	return ncfg.dbIndex
}

func (ncfg *networkConfiguration) SetIndex(index uint64) {
	ncfg.dbIndex = index
	ncfg.dbExists = true
}

func (ncfg *networkConfiguration) Exists() bool {
	return ncfg.dbExists
}

func (ncfg *networkConfiguration) Skip() bool {
	return false
}

func (ncfg *networkConfiguration) New() datastore.KVObject {
	return &networkConfiguration{}
}

func (ncfg *networkConfiguration) CopyTo(o datastore.KVObject) error {
	dstNcfg := o.(*networkConfiguration)
	*dstNcfg = *ncfg
	return nil
}

func (ep *bridgeEndpoint) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	epMap["id"] = ep.id
	epMap["nid"] = ep.nid
	epMap["SrcName"] = ep.srcName
	epMap["MacAddress"] = ep.macAddress.String()
	epMap["Addr"] = ep.addr.String()
	if ep.addrv6 != nil {
		epMap["Addrv6"] = ep.addrv6.String()
	}
	epMap["ContainerConfig"] = ep.containerConfig
	epMap["ExternalConnConfig"] = ep.extConnConfig
	epMap["PortMapping"] = ep.portMapping

	return json.Marshal(epMap)
}

func (ep *bridgeEndpoint) UnmarshalJSON(b []byte) error {
	var (
		err   error
		epMap map[string]interface{}
	)

	if err = json.Unmarshal(b, &epMap); err != nil {
		return fmt.Errorf("Failed to unmarshal to bridge endpoint: %v", err)
	}

	if v, ok := epMap["MacAddress"]; ok {
		if ep.macAddress, err = net.ParseMAC(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode bridge endpoint MAC address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	if v, ok := epMap["Addr"]; ok {
		if ep.addr, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode bridge endpoint IPv4 address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	if v, ok := epMap["Addrv6"]; ok {
		if ep.addrv6, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode bridge endpoint IPv6 address (%s) after json unmarshal: %v", v.(string), err)
		}
	}
	ep.id = epMap["id"].(string)
	ep.nid = epMap["nid"].(string)
	ep.srcName = epMap["SrcName"].(string)
	d, _ := json.Marshal(epMap["ContainerConfig"])
	if err := json.Unmarshal(d, &ep.containerConfig); err != nil {
		log.G(context.TODO()).Warnf("Failed to decode endpoint container config %v", err)
	}
	d, _ = json.Marshal(epMap["ExternalConnConfig"])
	if err := json.Unmarshal(d, &ep.extConnConfig); err != nil {
		log.G(context.TODO()).Warnf("Failed to decode endpoint external connectivity configuration %v", err)
	}
	d, _ = json.Marshal(epMap["PortMapping"])
	if err := json.Unmarshal(d, &ep.portMapping); err != nil {
		log.G(context.TODO()).Warnf("Failed to decode endpoint port mapping %v", err)
	}
	// Until release 27.0, HostPortEnd in PortMapping (operational data) was left at
	// the value it had in ExternalConnConfig.PortBindings (configuration). So, for
	// example, if the configured host port range was 8000-8009 and the allocated
	// port was 8004, the stored range was 8004-8009. Also, if allocation for an
	// explicit (non-ephemeral) range failed because some other process had a port
	// bound, there was no attempt to retry (because HostPort!=0). Now that's fixed,
	// on live-restore we don't want to allocate different ports - so, remove the range
	// from the operational data.
	// TODO(robmry) - remove once direct upgrade from moby 26.x is no longer supported.
	for i := range ep.portMapping {
		ep.portMapping[i].HostPortEnd = ep.portMapping[i].HostPort
	}

	return nil
}

func (ep *bridgeEndpoint) Key() []string {
	return []string{bridgeEndpointPrefix, ep.id}
}

func (ep *bridgeEndpoint) KeyPrefix() []string {
	return []string{bridgeEndpointPrefix}
}

func (ep *bridgeEndpoint) Value() []byte {
	b, err := json.Marshal(ep)
	if err != nil {
		return nil
	}
	return b
}

func (ep *bridgeEndpoint) SetValue(value []byte) error {
	return json.Unmarshal(value, ep)
}

func (ep *bridgeEndpoint) Index() uint64 {
	return ep.dbIndex
}

func (ep *bridgeEndpoint) SetIndex(index uint64) {
	ep.dbIndex = index
	ep.dbExists = true
}

func (ep *bridgeEndpoint) Exists() bool {
	return ep.dbExists
}

func (ep *bridgeEndpoint) Skip() bool {
	return false
}

func (ep *bridgeEndpoint) New() datastore.KVObject {
	return &bridgeEndpoint{}
}

func (ep *bridgeEndpoint) CopyTo(o datastore.KVObject) error {
	dstEp := o.(*bridgeEndpoint)
	*dstEp = *ep
	return nil
}

// restorePortAllocations is used during live-restore. It re-creates iptables
// forwarding/NAT rules, and restarts docker-proxy, as needed.
//
// TODO(robmry) - if any previously-mapped host ports are no longer available, all
// iptables forwarding/NAT rules get removed and there will be no docker-proxy
// processes. So, the container will be left running, but inaccessible.
func (n *bridgeNetwork) restorePortAllocations(ep *bridgeEndpoint) {
	if ep.extConnConfig == nil ||
		ep.extConnConfig.ExposedPorts == nil ||
		ep.extConnConfig.PortBindings == nil {
		return
	}

	// ep.portMapping has HostPort=HostPortEnd, the host port allocated last
	// time around ... use that in place of ep.extConnConfig.PortBindings, which
	// may specify host port ranges.
	cfg := make([]types.PortBinding, len(ep.portMapping))
	for i, b := range ep.portMapping {
		cfg[i] = b.PortBinding
	}

	var err error
	ep.portMapping, err = n.addPortMappings(ep.addr, ep.addrv6, cfg, n.config.DefaultBindingIP)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to reserve existing port mapping for endpoint %.7s:%v", ep.id, err)
	}
}
