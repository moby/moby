package bridge

import (
	"fmt"
	"math/rand"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/vishvananda/netlink"
)

func SetupDevice(i *Interface) error {
	// We only attempt to create the bridge when the requested device name is
	// the default one.
	if i.Config.BridgeName != DefaultBridgeName {
		return fmt.Errorf("bridge device with non default name %q must be created manually", i.Config.BridgeName)
	}

	// Set the Interface netlink.Bridge.
	i.Link = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: i.Config.BridgeName,
		},
	}

	// Only set the bridge's MAC address if the kernel version is > 3.3, as it
	// was not supported before that.
	kv, err := kernel.GetKernelVersion()
	if err == nil && (kv.Kernel >= 3 && kv.Major >= 3) {
		i.Link.Attrs().HardwareAddr = generateRandomMAC()
		log.Debugf("Setting bridge mac address to %s", i.Link.Attrs().HardwareAddr)
	}

	return netlink.LinkAdd(i.Link)
}

func SetupDeviceUp(i *Interface) error {
	return netlink.LinkSetUp(i.Link)
}

func generateRandomMAC() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	for i := 0; i < 6; i++ {
		hw[i] = byte(rand.Intn(255))
	}
	hw[0] &^= 0x1 // clear multicast bit
	hw[0] |= 0x2  // set local assignment bit (IEEE802)
	return hw
}
