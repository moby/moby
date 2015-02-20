package bridge

import "github.com/docker/libnetwork"

const networkType = "bridgednetwork"

func init() {
	libnetwork.RegisterNetworkType(networkType, Create)
}

func Create(options libnetwork.DriverParams) (libnetwork.Network, error) {
	return &bridgeNetwork{}, nil
}

type bridgeNetwork struct {
}

func (b *bridgeNetwork) Name() string {
	return ""
}

func (b *bridgeNetwork) Type() string {
	return networkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	return nil, nil
}
