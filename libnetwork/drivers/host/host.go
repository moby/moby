package host

import (
	"context"
	"sync"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
)

const NetworkType = "host"

type driver struct {
	network string
	sync.Mutex
}

func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(NetworkType, &driver{}, driverapi.Capability{
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

func (d *driver) EventNotify(_ driverapi.EventType, _, _, _ string, _ []byte) {
}

func (d *driver) DecodeTableEntry(_ string, _ string, _ []byte) (string, map[string]string) {
	return "", nil
}

func (d *driver) CreateNetwork(_ context.Context, id string, _ map[string]interface{}, _ driverapi.NetworkInfo, _, _ []driverapi.IPAMData) error {
	d.Lock()
	defer d.Unlock()

	if d.network != "" {
		return types.ForbiddenErrorf("only one instance of %q network is allowed", NetworkType)
	}

	d.network = id

	return nil
}

func (d *driver) DeleteNetwork(_ string) error {
	return types.ForbiddenErrorf("network of type %q cannot be deleted", NetworkType)
}

func (d *driver) CreateEndpoint(_ context.Context, _, _ string, _ driverapi.InterfaceInfo, _ map[string]interface{}) error {
	return nil
}

func (d *driver) DeleteEndpoint(_, _ string) error {
	return nil
}

func (d *driver) EndpointOperInfo(_, _ string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(_ context.Context, _, _ string, _ string, _ driverapi.JoinInfo, _, _ map[string]interface{}) error {
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(_, _ string) error {
	return nil
}

func (d *driver) ProgramExternalConnectivity(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(_, _ string) error {
	return nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
