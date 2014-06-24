// +build linux

package network

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

//  crosbymichael: could make a network strategy that instead of returning veth pair names it returns a pid to an existing network namespace
type NetNS struct {
}

func (v *NetNS) Create(n *Network, nspid int, context map[string]string) error {
	context["nspath"] = n.NsPath
	return nil
}

func (v *NetNS) Initialize(config *Network, context map[string]string) error {
	nspath, exists := context["nspath"]
	if !exists {
		return fmt.Errorf("nspath does not exist in network context")
	}
	f, err := os.OpenFile(nspath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace fd: %v", err)
	}
	if err := system.Setns(f.Fd(), syscall.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to setns current network namespace: %v", err)
	}
	return nil
}
