package ivmanager

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const networkType = "ipvlan"

type driver struct{}

// Register registers a new instance of the ipvlan manager driver.
func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(networkType, &driver{}, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Global,
	})
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateNetwork(ctx context.Context, id string, option map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]any) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]any, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, _ map[string]any) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) Leave(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
