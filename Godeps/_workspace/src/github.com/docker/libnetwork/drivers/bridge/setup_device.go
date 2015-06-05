package bridge

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

// SetupDevice create a new bridge interface/
func setupDevice(config *NetworkConfiguration, i *bridgeInterface) error {
	// We only attempt to create the bridge when the requested device name is
	// the default one.
	if config.BridgeName != DefaultBridgeName && !config.AllowNonDefaultBridge {
		return NonDefaultBridgeExistError(config.BridgeName)
	}

	// Set the bridgeInterface netlink.Bridge.
	i.Link = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: config.BridgeName,
		},
	}

	// Only set the bridge's MAC address if the kernel version is > 3.3, as it
	// was not supported before that.
	kv, err := kernel.GetKernelVersion()
	if err == nil && (kv.Kernel >= 3 && kv.Major >= 3) {
		i.Link.Attrs().HardwareAddr = netutils.GenerateRandomMAC()
		log.Debugf("Setting bridge mac address to %s", i.Link.Attrs().HardwareAddr)
	}

	// Call out to netlink to create the device.
	return netlink.LinkAdd(i.Link)
}

// SetupDeviceUp ups the given bridge interface.
func setupDeviceUp(config *NetworkConfiguration, i *bridgeInterface) error {
	err := netlink.LinkSetUp(i.Link)
	if err != nil {
		return err
	}

	// Attempt to update the bridge interface to refresh the flags status,
	// ignoring any failure to do so.
	if lnk, err := netlink.LinkByName(config.BridgeName); err == nil {
		i.Link = lnk
	}
	return nil
}
