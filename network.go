package docker

import (
	"net"
)

const (
	networkGateway   = "10.0.3.1"
	networkPrefixLen = 24
)

type NetworkInterface struct {
	IpAddress   string
	IpPrefixLen int
	Gateway     net.IP
}

func allocateIPAddress() string {
	return "10.0.3.2"
}

func allocateNetwork() (*NetworkInterface, error) {
	iface := &NetworkInterface{
		IpAddress:   allocateIPAddress(),
		IpPrefixLen: networkPrefixLen,
		Gateway:     net.ParseIP(networkGateway),
	}
	return iface, nil
}
