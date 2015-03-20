package bridge

import (
	"github.com/docker/libnetwork"
	"github.com/vishvananda/netlink"
)

type bridgeNetwork struct {
	NetworkName string
	bridge      *bridgeInterface
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

func (b *bridgeNetwork) Delete() error {
	return netlink.LinkDel(b.bridge.Link)
}
