// Package overlayutils provides utility functions for overlay networks
package overlayutils

import (
	"fmt"
	"sync"
)

var (
	mutex        sync.RWMutex
	vxlanUDPPort uint32
)

const defaultVXLANUDPPort = 4789

func init() {
	vxlanUDPPort = defaultVXLANUDPPort
}

// ConfigVXLANUDPPort configures vxlan udp port number.
func ConfigVXLANUDPPort(vxlanPort uint32) error {
	// if the value comes as 0 by any reason we set it to default value 4789
	if vxlanPort == 0 {
		vxlanPort = defaultVXLANUDPPort
	}
	// IANA procedures for each range in detail
	// The Well Known Ports, aka the System Ports, from 0-1023
	// The Registered Ports, aka the User Ports, from 1024-49151
	// The Dynamic Ports, aka the Private Ports, from 49152-65535
	// So we can allow range between 1024 to 49151
	if vxlanPort < 1024 || vxlanPort > 49151 {
		return fmt.Errorf("ConfigVxlanUDPPort Vxlan UDP port number is not in valid range %d", vxlanPort)
	}
	mutex.Lock()
	vxlanUDPPort = vxlanPort
	mutex.Unlock()
	return nil
}

// VXLANUDPPort returns Vxlan UDP port number
func VXLANUDPPort() uint32 {
	mutex.RLock()
	defer mutex.RUnlock()
	return vxlanUDPPort
}
