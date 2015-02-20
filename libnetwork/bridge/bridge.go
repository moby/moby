package bridge

import (
	"net"

	"github.com/docker/libnetwork"
)

const networkType = "bridgednetwork"

type bridgeConfiguration struct {
	Subnet net.IPNet
}

func init() {
	libnetwork.RegisterNetworkType(networkType, Create, bridgeConfiguration{})
}

func Create(config *bridgeConfiguration) (libnetwork.Network, error) {
	return &bridgeNetwork{Config: *config}, nil
}

type bridgeNetwork struct {
	Config bridgeConfiguration
}

func (b *bridgeNetwork) Type() string {
	return networkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	return nil, nil
}
