package bridge

import "github.com/docker/libnetwork"

type bridgeNetwork struct {
	Config      Configuration
	NetworkName string
}

func (b *bridgeNetwork) Name() string {
	return b.NetworkName
}

func (b *bridgeNetwork) Type() string {
	return networkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	// TODO
	return nil, nil
}
