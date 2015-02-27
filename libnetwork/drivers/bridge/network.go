package bridge

import "github.com/docker/libnetwork"

type bridgeNetwork struct {
	Config Configuration
}

func (b *bridgeNetwork) Type() string {
	return NetworkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	// TODO
	return nil, nil
}
