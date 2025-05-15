package cnmallocator

import (
	"context"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
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

func (d *manager) NetworkAllocate(_ string, _ map[string]string, _, _ []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) NetworkFree(_ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateNetwork(_ context.Context, _ string, _ map[string]interface{}, _ driverapi.NetworkInfo, _, _ []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) EventNotify(_ driverapi.EventType, _, _, _ string, _ []byte) {
}

func (d *manager) DecodeTableEntry(_ string, _ string, _ []byte) (string, map[string]string) {
	return "", nil
}

func (d *manager) DeleteNetwork(_ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) CreateEndpoint(_ context.Context, _, _ string, _ driverapi.InterfaceInfo, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) DeleteEndpoint(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) EndpointOperInfo(_, _ string) (map[string]interface{}, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *manager) Join(_ context.Context, _, _ string, _ string, _ driverapi.JoinInfo, _, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) Leave(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) Type() string {
	return d.networkType
}

func (d *manager) IsBuiltIn() bool {
	return true
}

func (d *manager) ProgramExternalConnectivity(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *manager) RevokeExternalConnectivity(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}
