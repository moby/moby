package brmanager

import (
	"context"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
)

const networkType = "bridge"

type driver struct{}

// Register registers a new instance of the bridge manager driver with r.
func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(networkType, &driver{}, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Local,
	})
}

func (d *driver) NetworkAllocate(_ string, _ map[string]string, _, _ []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(_ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateNetwork(_ context.Context, _ string, _ map[string]interface{}, _ driverapi.NetworkInfo, _, _ []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EventNotify(_ driverapi.EventType, _, _, _ string, _ []byte) {
}

func (d *driver) DecodeTableEntry(_ string, _ string, _ []byte) (string, map[string]string) {
	return "", nil
}

func (d *driver) DeleteNetwork(_ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateEndpoint(_ context.Context, _, _ string, _ driverapi.InterfaceInfo, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteEndpoint(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(_, _ string) (map[string]interface{}, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) Join(_ context.Context, _, _ string, _ string, _ driverapi.JoinInfo, _, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) Leave(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func (d *driver) ProgramExternalConnectivity(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) RevokeExternalConnectivity(_, _ string) error {
	return types.NotImplementedErrorf("not implemented")
}
