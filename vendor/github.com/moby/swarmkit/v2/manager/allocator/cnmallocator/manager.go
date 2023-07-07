package cnmallocator

import (
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/types"
)

type manager struct {
	networkType string
}

// RegisterManager registers a new instance of the manager driver for networkType with r.
func RegisterManager(r driverapi.Registerer, networkType string) error {
	return r.RegisterDriver(networkType, &manager{networkType: networkType}, driverapi.Capability{
		DataScope:         datastore.LocalScope,
		ConnectivityScope: datastore.LocalScope,
	})
}

func (d *manager) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *manager) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}

func (d *manager) DeleteNetwork(nid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) DeleteEndpoint(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) Leave(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) Type() string {
	return d.networkType
}

func (d *manager) IsBuiltIn() bool {
	return true
}

func (d *manager) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) RevokeExternalConnectivity(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}
