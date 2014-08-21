// +build linux

package network

import (
	"fmt"
	"os"
	"syscall"

	"github.com/docker/libcontainer/system"
)

//  crosbymichael: could make a network strategy that instead of returning veth pair names it returns a pid to an existing network namespace
type NetNS struct {
}

func (v *NetNS) Create(n *Network, nspid int, networkState *NetworkState) error {
	networkState.NsPath = n.NsPath
	return nil
}

func (v *NetNS) Initialize(config *Network, networkState *NetworkState) error {
	if networkState.NsPath == "" {
		return fmt.Errorf("nspath does is not specified in NetworkState")
	}

	f, err := os.OpenFile(networkState.NsPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace fd: %v", err)
	}

	if err := system.Setns(f.Fd(), syscall.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to setns current network namespace: %v", err)
	}

	return nil
}
