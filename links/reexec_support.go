package links

import (
	"fmt"
	"os"
	"syscall"

	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/system"
)

type NetworkSettings struct {
	DeviceName string
	IpNet      string
	Gateway    string
}

func netNamespace(pid string) (*os.File, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%s/ns/net", pid))
	if err != nil {
		return nil, err
	}

	return f, nil
}

func moveVethDevice(enterFd *os.File, targetFd *os.File, netSettings NetworkSettings) func() error {
	return func() error {
		if err := network.InterfaceDown(netSettings.DeviceName); err != nil {
			return err
		}

		if err := network.SetInterfaceInNamespaceFd(netSettings.DeviceName, targetFd.Fd()); err != nil {
			return err
		}

		if err := system.Setns(targetFd.Fd(), syscall.CLONE_NEWNET); err != nil {
			return err
		}

		if err := network.SetInterfaceIp(netSettings.DeviceName, netSettings.IpNet); err != nil {
			return err
		}

		if err := network.SetDefaultGateway(netSettings.Gateway, netSettings.DeviceName); err != nil {
			return err
		}

		if err := network.InterfaceUp(netSettings.DeviceName); err != nil {
			return err
		}

		return nil
	}
}
