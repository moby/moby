package bridge

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
)

const bridgePrefix = "bridge"

func (d *driver) initStore(option map[string]interface{}) error {
	if data, ok := option[netlabel.LocalKVClient]; ok {
		var err error
		dsc, ok := data.(discoverapi.DatastoreConfigData)
		if !ok {
			return types.InternalErrorf("incorrect data in datastore configuration: %v", data)
		}
		d.store, err = datastore.NewDataStoreFromConfig(dsc)
		if err != nil {
			return types.InternalErrorf("bridge driver failed to initialize data store: %v", err)
		}

		return d.populateNetworks()
	}

	return nil
}

func (d *driver) populateNetworks() error {
	kvol, err := d.store.List(datastore.Key(bridgePrefix), &networkConfiguration{})
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
			logrus.Warnf("could not create bridge network for id %s bridge name %s while booting up from persistent state", ncfg.ID, ncfg.BridgeName)
		}
	}

	return nil
}

func (d *driver) storeUpdate(kvObject datastore.KVObject) error {
	if d.store == nil {
		logrus.Warnf("bridge store not initialized. kv object %s is not added to the store", datastore.Key(kvObject.Key()...))
		return nil
	}

	if err := d.store.PutObjectAtomic(kvObject); err != nil {
		return fmt.Errorf("failed to update bridge store for object type %T: %v", kvObject, err)
	}

	return nil
}

func (d *driver) storeDelete(kvObject datastore.KVObject) error {
	if d.store == nil {
		logrus.Debugf("bridge store not initialized. kv object %s is not deleted from store", datastore.Key(kvObject.Key()...))
		return nil
	}

retry:
	if err := d.store.DeleteObjectAtomic(kvObject); err != nil {
		if err == datastore.ErrKeyModified {
			if err := d.store.GetObject(datastore.Key(kvObject.Key()...), kvObject); err != nil {
				return fmt.Errorf("could not update the kvobject to latest when trying to delete: %v", err)
			}
			goto retry
		}
		return err
	}

	return nil
}

func (ncfg *networkConfiguration) MarshalJSON() ([]byte, error) {
	nMap := make(map[string]interface{})
	nMap["ID"] = ncfg.ID
	nMap["BridgeName"] = ncfg.BridgeName
	nMap["EnableIPv6"] = ncfg.EnableIPv6
	nMap["EnableIPMasquerade"] = ncfg.EnableIPMasquerade
	nMap["EnableICC"] = ncfg.EnableICC
	nMap["Mtu"] = ncfg.Mtu
	nMap["Internal"] = ncfg.Internal
	nMap["DefaultBridge"] = ncfg.DefaultBridge
	nMap["DefaultBindingIP"] = ncfg.DefaultBindingIP.String()
	nMap["DefaultGatewayIPv4"] = ncfg.DefaultGatewayIPv4.String()
	nMap["DefaultGatewayIPv6"] = ncfg.DefaultGatewayIPv6.String()

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

	ncfg.DefaultBridge = nMap["DefaultBridge"].(bool)
	ncfg.DefaultBindingIP = net.ParseIP(nMap["DefaultBindingIP"].(string))
	ncfg.DefaultGatewayIPv4 = net.ParseIP(nMap["DefaultGatewayIPv4"].(string))
	ncfg.DefaultGatewayIPv6 = net.ParseIP(nMap["DefaultGatewayIPv6"].(string))
	ncfg.ID = nMap["ID"].(string)
	ncfg.BridgeName = nMap["BridgeName"].(string)
	ncfg.EnableIPv6 = nMap["EnableIPv6"].(bool)
	ncfg.EnableIPMasquerade = nMap["EnableIPMasquerade"].(bool)
	ncfg.EnableICC = nMap["EnableICC"].(bool)
	ncfg.Mtu = int(nMap["Mtu"].(float64))
	if v, ok := nMap["Internal"]; ok {
		ncfg.Internal = v.(bool)
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
	return ncfg.DefaultBridge
}

func (ncfg *networkConfiguration) New() datastore.KVObject {
	return &networkConfiguration{}
}

func (ncfg *networkConfiguration) CopyTo(o datastore.KVObject) error {
	dstNcfg := o.(*networkConfiguration)
	*dstNcfg = *ncfg
	return nil
}

func (ncfg *networkConfiguration) DataScope() string {
	return datastore.LocalScope
}
