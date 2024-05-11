package null

import (
	"context"
	"sync"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
)

const NetworkType = "null"

type driver struct {
	network string
	sync.Mutex
}

// Register registers a new instance of the null driver.
func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(NetworkType, &driver{}, driverapi.Capability{
		DataScope: scope.Local,
	})
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	d.Lock()
	defer d.Unlock()

	if d.network != "" {
		return types.ForbiddenErrorf("only one instance of %q network is allowed", NetworkType)
	}

	d.network = id

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.ForbiddenErrorf("network of type %q cannot be deleted", NetworkType)
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]interface{}) error {
	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	return nil
}

func (d *driver) ProgramExternalConnectivity(_ context.Context, nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
