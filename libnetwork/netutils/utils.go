// Network utility functions.

package netutils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/docker/docker/libnetwork/types"
)

var (
	// ErrNetworkOverlapsWithNameservers preformatted error
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	// ErrNetworkOverlaps preformatted error
	ErrNetworkOverlaps = errors.New("requested network overlaps with existing network")
)

// CheckNameserverOverlaps checks whether the passed network overlaps with any of the nameservers
func CheckNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	if len(nameservers) > 0 {
		for _, ns := range nameservers {
			_, nsNetwork, err := net.ParseCIDR(ns)
			if err != nil {
				return err
			}
			if NetworkOverlaps(toCheck, nsNetwork) {
				return ErrNetworkOverlapsWithNameservers
			}
		}
	}
	return nil
}

// NetworkOverlaps detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	return netX.Contains(netY.IP) || netY.Contains(netX.IP)
}

// NetworkRange calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	if network == nil {
		return nil, nil
	}

	firstIP := network.IP.Mask(network.Mask)
	lastIP := types.GetIPCopy(firstIP)
	for i := 0; i < len(firstIP); i++ {
		lastIP[i] = firstIP[i] | ^network.Mask[i]
	}

	if network.IP.To4() != nil {
		firstIP = firstIP.To4()
		lastIP = lastIP.To4()
	}

	return firstIP, lastIP
}

func genMAC(ip net.IP) net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	// The first byte of the MAC address has to comply with these rules:
	// 1. Unicast: Set the least-significant bit to 0.
	// 2. Address is locally administered: Set the second-least-significant bit (U/L) to 1.
	hw[0] = 0x02
	// The first 24 bits of the MAC represent the Organizationally Unique Identifier (OUI).
	// Since this address is locally administered, we can do whatever we want as long as
	// it doesn't conflict with other addresses.
	hw[1] = 0x42
	// Fill the remaining 4 bytes based on the input
	if ip == nil {
		rand.Read(hw[2:])
	} else {
		copy(hw[2:], ip.To4())
	}
	return hw
}

// GenerateRandomMAC returns a new 6-byte(48-bit) hardware address (MAC)
func GenerateRandomMAC() net.HardwareAddr {
	return genMAC(nil)
}

// GenerateMACFromIP returns a locally administered MAC address where the 4 least
// significant bytes are derived from the IPv4 address.
func GenerateMACFromIP(ip net.IP) net.HardwareAddr {
	return genMAC(ip)
}

// GenerateRandomName returns a string of the specified length, created by joining the prefix to random hex characters.
// The length must be strictly larger than len(prefix), or an error will be returned.
func GenerateRandomName(prefix string, length int) (string, error) {
	if length <= len(prefix) {
		return "", fmt.Errorf("invalid length %d for prefix %s", length, prefix)
	}

	// We add 1 here as integer division will round down, and we want to round up.
	b := make([]byte, (length-len(prefix)+1)/2)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	// By taking a slice here, we ensure that the string is always the correct length.
	return (prefix + hex.EncodeToString(b))[:length], nil
}

// ReverseIP accepts a V4 or V6 IP string in the canonical form and returns a reversed IP in
// the dotted decimal form . This is used to setup the IP to service name mapping in the optimal
// way for the DNS PTR queries.
func ReverseIP(IP string) string {
	var reverseIP []string

	if net.ParseIP(IP).To4() != nil {
		reverseIP = strings.Split(IP, ".")
		l := len(reverseIP)
		for i, j := 0, l-1; i < l/2; i, j = i+1, j-1 {
			reverseIP[i], reverseIP[j] = reverseIP[j], reverseIP[i]
		}
	} else {
		reverseIP = strings.Split(IP, ":")

		// Reversed IPv6 is represented in dotted decimal instead of the typical
		// colon hex notation
		for key := range reverseIP {
			if len(reverseIP[key]) == 0 { // expand the compressed 0s
				reverseIP[key] = strings.Repeat("0000", 8-strings.Count(IP, ":"))
			} else if len(reverseIP[key]) < 4 { // 0-padding needed
				reverseIP[key] = strings.Repeat("0", 4-len(reverseIP[key])) + reverseIP[key]
			}
		}

		reverseIP = strings.Split(strings.Join(reverseIP, ""), "")

		l := len(reverseIP)
		for i, j := 0, l-1; i < l/2; i, j = i+1, j-1 {
			reverseIP[i], reverseIP[j] = reverseIP[j], reverseIP[i]
		}
	}

	return strings.Join(reverseIP, ".")
}
