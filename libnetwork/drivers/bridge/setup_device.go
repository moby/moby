//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

// SetupDevice create a new bridge interface/
func setupDevice(config *networkConfiguration, i *bridgeInterface) error {
	// We only attempt to create the bridge when the requested device name is
	// the default one. The default bridge name can be overridden with the
	// DOCKER_TEST_CREATE_DEFAULT_BRIDGE env var. It should be used only for
	// test purpose.
	var defaultBridgeName string
	if defaultBridgeName = os.Getenv("DOCKER_TEST_CREATE_DEFAULT_BRIDGE"); defaultBridgeName == "" {
		defaultBridgeName = DefaultBridgeName
	}
	if config.BridgeName != defaultBridgeName && config.DefaultBridge {
		return NonDefaultBridgeExistError(config.BridgeName)
	}

	// Set the bridgeInterface netlink.Bridge.
	i.Link = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: config.BridgeName,
		},
	}

	// Set the bridge's MAC address. Requires kernel version 3.3 or up.
	hwAddr := netutils.GenerateRandomMAC()
	i.Link.Attrs().HardwareAddr = hwAddr
	log.G(context.TODO()).Debugf("Setting bridge mac address to %s", hwAddr)

	if err := i.nlh.LinkAdd(i.Link); err != nil {
		log.G(context.TODO()).WithError(err).Errorf("Failed to create bridge %s via netlink", config.BridgeName)
		return err
	}

	return nil
}

func setupDefaultSysctl(config *networkConfiguration, i *bridgeInterface) error {
	// Disable IPv6 router advertisements originating on the bridge
	sysPath := filepath.Join("/proc/sys/net/ipv6/conf/", config.BridgeName, "accept_ra")
	if _, err := os.Stat(sysPath); err != nil {
		log.G(context.TODO()).
			WithField("bridge", config.BridgeName).
			WithField("syspath", sysPath).
			Info("failed to read ipv6 net.ipv6.conf.<bridge>.accept_ra")
		return nil
	}
	if err := os.WriteFile(sysPath, []byte{'0', '\n'}, 0644); err != nil {
		log.G(context.TODO()).WithError(err).Warn("unable to disable IPv6 router advertisement")
	}
	return nil
}

// SetupDeviceUp ups the given bridge interface.
func setupDeviceUp(config *networkConfiguration, i *bridgeInterface) error {
	err := i.nlh.LinkSetUp(i.Link)
	if err != nil {
		return fmt.Errorf("Failed to set link up for %s: %v", config.BridgeName, err)
	}

	// Attempt to update the bridge interface to refresh the flags status,
	// ignoring any failure to do so.
	if lnk, err := i.nlh.LinkByName(config.BridgeName); err == nil {
		i.Link = lnk
	} else {
		log.G(context.TODO()).Warnf("Failed to retrieve link for interface (%s): %v", config.BridgeName, err)
	}
	return nil
}
