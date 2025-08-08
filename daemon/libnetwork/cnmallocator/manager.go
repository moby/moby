package cnmallocator

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

type manager struct {
	networkType string
}

// RegisterManager registers a new instance of the manager driver for networkType with r.
func RegisterManager(r driverapi.Registerer, networkType string) error {
	return r.RegisterDriver(networkType, &manager{networkType: networkType}, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Local,
	})
}

func (d *manager) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateNetwork(ctx context.Context, id string, option map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) DeleteNetwork(nid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]any) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) DeleteEndpoint(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) EndpointOperInfo(nid, eid string) (map[string]any, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, _ map[string]any) error {
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
