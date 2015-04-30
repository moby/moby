// +build windows

package windowsnetwork

import (
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
)

const (
	DefaultNetworkBridge = "Virtual Switch"
)

// TODO Windows. This networking driver contains interim code, especially
// in regard to the MAC allocation, which eventually will be pushed down to
// the HCS. However, it is sufficient for the initial bring-up of Windows
// containers.

// Network interface represents the networking stack of a container
type networkInterface struct {
	MACAddress net.HardwareAddr
}

type ifaces struct {
	c map[string]*networkInterface
	sync.Mutex
}

func (i *ifaces) Set(key string, n *networkInterface) {
	i.Lock()
	i.c[key] = n
	i.Unlock()
}

func (i *ifaces) Get(key string) *networkInterface {
	i.Lock()
	res := i.c[key]
	i.Unlock()
	return res
}

var (
	bridgeIface       string
	currentInterfaces = ifaces{c: make(map[string]*networkInterface)}
)

func InitDriver(config *Config) error {
	if err := SetupMACRange([]byte{0x02, 0x42}); err != nil {
		return err
	}

	bridgeIface = config.Iface
	if bridgeIface == "" {
		bridgeIface = DefaultNetworkBridge
	}

	return nil
}

// Allocate a network interface
func Allocate(id, requestedMac, requestedIP, requestedIPv6 string) (*network.Settings, error) {
	var (
		mac net.HardwareAddr
		err error
	)

	// If no explicit mac address was given, generate a random one.
	if mac, err = net.ParseMAC(requestedMac); err != nil {
		if mac, err = RequestMAC(); err != nil {
			return nil, err
		}
	}
	logrus.Debugln("NetworkDriver-Windows MAC=", mac.String())

	networkSettings := &network.Settings{
		MacAddress: mac.String(),
		Bridge:     bridgeIface,
	}

	currentInterfaces.Set(id, &networkInterface{
		MACAddress: mac,
	})

	return networkSettings, nil
}

// Release an interface
// FIXME With engine factor out, Release() no longer returns an error. Should it?
func Release(id string) {
	containerInterface := currentInterfaces.Get(id)

	if containerInterface == nil {
		logrus.Infof("No network information to release for %s", id)
	}

	if err := ReleaseMac(containerInterface.MACAddress); err != nil {
		logrus.Infof("Unable to release MAC Address %s %s", containerInterface.MACAddress.String(), err)
	}
	return
}
