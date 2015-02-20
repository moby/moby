package bridge

import (
	"net"

	"github.com/docker/libnetwork"
)

const networkType = "bridgednetwork"

func init() {
	libnetwork.RegisterNetworkType(networkType, Create)
}

func Create(options libnetwork.strategyParams) libnetwork.Network {
	return &bridgeNetwork{}
}

type Configuration struct {
	Subnet net.IPNet
}

type bridgeNetwork struct {
	Config Configuration
}

func (b *bridgeNetwork) Name() string {
	return b.Id
}

func (b *bridgeNetwork) Type() string {
	return networkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	return nil, nil
}
