// Package overlayutils provides utility functions for overlay networks
package overlayutils

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var (
	mutex        sync.RWMutex
	vxlanUDPPort = defaultVXLANUDPPort
)

const defaultVXLANUDPPort uint32 = 4789

// ConfigVXLANUDPPort configures the VXLAN UDP port (data path port) number.
// If no port is set, the default (4789) is returned. Valid port numbers are
// between 1024 and 49151.
func ConfigVXLANUDPPort(vxlanPort uint32) error {
	if vxlanPort == 0 {
		vxlanPort = defaultVXLANUDPPort
	}
	// IANA procedures for each range in detail
	// The Well Known Ports, aka the System Ports, from 0-1023
	// The Registered Ports, aka the User Ports, from 1024-49151
	// The Dynamic Ports, aka the Private Ports, from 49152-65535
	// So we can allow range between 1024 to 49151
	if vxlanPort < 1024 || vxlanPort > 49151 {
		return fmt.Errorf("VXLAN UDP port number is not in valid range (1024-49151): %d", vxlanPort)
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

// AppendVNIList appends the VNI values encoded as a CSV string to slice.
func AppendVNIList(vnis []uint32, csv string) ([]uint32, error) {
	for {
		var (
			vniStr string
			found  bool
		)
		vniStr, csv, found = strings.Cut(csv, ",")
		vni, err := strconv.Atoi(vniStr)
		if err != nil {
			return vnis, fmt.Errorf("invalid vxlan id value %q passed", vniStr)
		}

		vnis = append(vnis, uint32(vni))
		if !found {
			return vnis, nil
		}
	}
}
