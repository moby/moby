// +build windows

package windowsnetwork

import (
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
)

// We use a variation on Hyper-V semantics for MAC address allocation.
// This code currently places a limit at 65536 available MACs.
// The range is calculated as one of the following:
//
// OUI Prefix length 2:  OUI Byte 1
//                       OUI Byte 2
//                       3rd octet of first enumerated NIC first non-nil IPv4
//                       4th octet of first enumerated NIC first non-nil IPv4
//                       00:00 to FF:FF (Poolsize 65536)
//
// OUI Prefix length 3:  OUI Byte 1
//                       OUI Byte 2
//                       OUI Byte 3
//                       4th octet of first enumerated NIC first non-nil IPv4
//                       00:00 to FF:FF (Poolsize 65536)
//
// OUI Prefix length 4:  OUI Byte 1
//                       OUI Byte 2
//                       OUI Byte 3
//                       OUI Byte 4
//                       4th octet of first enumerated NIC first non-nil IPv4
//                       00 to FF  (Poolsize 256)

var (
	mutex sync.Mutex

	macBase     net.HardwareAddr
	endMACRange int32
	m           = make(map[int32]string)
	last        int32

	ErrAllMacsAllocated = errors.New("all MAC addresses are allocated")
	ErrInvalidOUI       = errors.New("Invalid OUI prefix")
	ErrInvalidMAC       = errors.New("Invalid MAC")
	ErrMACNotAllocated  = errors.New("MAC not allocated")
)

// randomByte returns a single random byte
func randomByte() byte {
	for {
		id := make([]byte, 1)
		if _, err := io.ReadFull(rand.Reader, id); err != nil {
			panic(err) // This shouldn't happen
		}
		return id[0]
	}
}

// getOctets get lowest two octets of the first v4 IP address enumerated, or
// two random digits in the case of error.
//
// TODO This is not perfect! Bugs below. However, this code will eventually
// be pushed down to the HCS layer and be removed from docker. It is sufficient
// for the initial bringup of Windows containers.
//
// - FlagUp bug in Windows. Always returns 0. Believe it's because
//   WSAIoctl SIO_GET_INTERFACE_LIST doesn't set iiFlags to IFF_UP
//   on a synthNIC. Hit this if running if i.Flags&net.FlagUp == net.FlagUp
//   which would be better code instead of checking for zero IP address.
//
// - Checking for zero IP address is incorrect.
//
// - IPv6 addresses. They are never returned. See interface_windows.go in GOLANG:
// .... GetAdaptersInfo returns IP_ADAPTER_INFO that
// .... contains IPv4 address list only. We should use another API
// .... for fetching IPv6 stuff from the kernel.
func getOctets() (octet1 byte, octet2 byte) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return randomByte(), randomByte()
	}
	for _, i := range ifaces {
		if addrs, err := i.Addrs(); err == nil {
			for _, addr := range addrs {
				switch addr.(type) {
				case *net.IPAddr:
					if ip := net.ParseIP(addr.String()); ip != nil {
						if ip.String() != "0.0.0.0" {
							if v4addr := net.IP.To4(ip); v4addr != nil {
								if len(ip) == 16 {
									logrus.Debugf("Seed MAC using %d.%d from %s on %s", ip[14], ip[15], ip.String(), i.HardwareAddr)
									return ip[14], ip[15]
								}
							}
						}
					}
				}
			}
		}
	}
	logrus.Warningln("Failed to find an IPv4 address to set MAC range. Using random seed instead")
	return randomByte(), randomByte()
}

// SetupMACRange is called by the driver in it's init processing
// Docker uses 02:42 as it's prefix in the documentation.
// For reference, Virtual Server was 00:03:FF
func SetupMACRange(ouiPrefix []byte) (err error) {
	if len(ouiPrefix) < 2 || len(ouiPrefix) > 4 {
		return ErrInvalidOUI
	}
	o1, o2 := getOctets()
	switch len(ouiPrefix) {
	case 2:
		macBase = append(ouiPrefix, o1, o2)
		endMACRange = (1 << 16) - 1
	case 3:
		macBase = append(ouiPrefix, o2)
		endMACRange = (1 << 16) - 1
	case 4:
		macBase = append(ouiPrefix, o2)
		endMACRange = (1 << 8) - 1
	}
	last = endMACRange
	logrus.Debugf("MAC address range %v:... addresses %d", macBase, endMACRange)
	return nil
}

// RequestMAC requests the next available MAC address.
func RequestMAC() (net.HardwareAddr, error) {
	mutex.Lock()
	defer mutex.Unlock()
	mac, err := findMac()
	if err != nil {
		return nil, err
	}
	ret := macBase
	for i := 0; i < 6-len(macBase); i++ {
		shift := (uint)(6-len(macBase)-i-1) * 8
		ret = append(ret, (byte)(mac>>shift&0xFF))
	}
	m[mac] = ret.String()
	return ret, err
}

// ReleaseMac releases a MAC address from the pool
func ReleaseMac(mac net.HardwareAddr) error {
	mutex.Lock()
	defer mutex.Unlock()
	if len(mac) != 6 {
		return ErrInvalidMAC
	}
	var index int32
	for i := 0; i < 6-len(macBase); i++ {
		bitshift := uint(6-len(macBase)-i-1) * 8
		index += int32(mac[i+len(macBase)]) << bitshift
	}
	if m[index] == "" {
		return ErrMACNotAllocated
	}
	delete(m, index)
	return nil
}

// ReleaseAll releases all ports for all ips.
func ReleaseAll() error {
	mutex.Lock()
	m = make(map[int32]string)
	last = 0
	mutex.Unlock()
	return nil
}

func findMac() (int32, error) {
	mac := last
	var i int32
	for i = 0; i <= endMACRange; i++ {
		mac++
		if mac > endMACRange {
			mac = 0
		}
		if _, ok := m[mac]; !ok {
			last = mac
			return mac, nil
		}
	}
	return 0, ErrAllMacsAllocated
}
