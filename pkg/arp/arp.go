package arp

import (
	"errors"
	"net"
)

const (
	ARP_HDRLEN  = 28
	ARP_REQUEST = 1
)

var (
	ErrUnsupported = errors.New("Unsupported OS/Arch")
)

func NewARP(ifaceName string) (*ARP, error) {
	// Find the index for the given interface
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}

	// Retrieve the first ip for source
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	srcIP, _, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		return nil, err
	}

	return &ARP{
		HType:      1,
		PType:      ETH_P_IP,
		HLen:       6,
		PLen:       4,
		OpCode:     ARP_REQUEST,
		SenderMac:  net.HardwareAddr(iface.HardwareAddr),
		SenderIP:   srcIP.To4(),
		TargetMac:  net.HardwareAddr([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}), // broadcast
		ifaceIndex: iface.Index,
	}, nil
}

type ARP struct {
	HType      uint16
	PType      uint16
	HLen       uint8
	PLen       uint8
	OpCode     uint16
	SenderMac  net.HardwareAddr
	SenderIP   net.IP
	TargetMac  net.HardwareAddr
	TargetIP   net.IP
	ifaceIndex int
}
