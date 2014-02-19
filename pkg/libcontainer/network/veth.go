package network

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os"
	"syscall"
)

// SetupVeth sets up an existing network namespace with the specified
// network configuration.
func SetupVeth(config *libcontainer.Network) error {
	if err := InterfaceDown(config.TempVethName); err != nil {
		return fmt.Errorf("interface down %s %s", config.TempVethName, err)
	}
	if err := ChangeInterfaceName(config.TempVethName, "eth0"); err != nil {
		return fmt.Errorf("change %s to eth0 %s", config.TempVethName, err)
	}
	if err := SetInterfaceIp("eth0", config.IP); err != nil {
		return fmt.Errorf("set eth0 ip %s", err)
	}

	if err := SetMtu("eth0", config.Mtu); err != nil {
		return fmt.Errorf("set eth0 mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("eth0"); err != nil {
		return fmt.Errorf("eth0 up %s", err)
	}

	if err := SetMtu("lo", config.Mtu); err != nil {
		return fmt.Errorf("set lo mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}

	if config.Gateway != "" {
		if err := SetDefaultGateway(config.Gateway); err != nil {
			return fmt.Errorf("set gateway to %s %s", config.Gateway, err)
		}
	}
	return nil
}

// SetupNamespaceMountDir prepares a new root for use as a mount
// source for bind mounting namespace fd to an outside path
func SetupNamespaceMountDir(root string) error {
	if err := os.MkdirAll(root, 0666); err != nil {
		return err
	}
	// make sure mounts are not unmounted by other mnt namespaces
	if err := syscall.Mount("", root, "none", syscall.MS_SHARED|syscall.MS_REC, ""); err != nil && err != syscall.EINVAL {
		return err
	}
	if err := syscall.Mount(root, root, "none", syscall.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

// DeleteNetworkNamespace unmounts the binding path and removes the
// file so that no references to the fd are present and the network
// namespace is automatically cleaned up
func DeleteNetworkNamespace(bindingPath string) error {
	if err := syscall.Unmount(bindingPath, 0); err != nil {
		return err
	}
	return os.Remove(bindingPath)
}
