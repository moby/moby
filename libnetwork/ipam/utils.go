package ipam

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/types"
)

type ipVersion int

const (
	v4 = 4
	v6 = 6
)

func getAddressRange(pool string) (*AddressRange, error) {
	ip, nw, err := net.ParseCIDR(pool)
	if err != nil {
		return nil, ipamapi.ErrInvalidSubPool
	}
	lIP, e := types.GetHostPartIP(nw.IP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's lowest ip address: %v", e)
	}
	bIP, e := types.GetBroadcastIP(nw.IP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's broadcast ip address: %v", e)
	}
	hIP, e := types.GetHostPartIP(bIP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's highest ip address: %v", e)
	}
	nw.IP = ip
	return &AddressRange{nw, ipToUint32(types.GetMinimalIP(lIP)), ipToUint32(types.GetMinimalIP(hIP))}, nil
}

func initLocalPredefinedPools() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 274)
	mask := []byte{255, 255, 0, 0}
	for i := 17; i < 32; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, byte(i), 0, 0}, Mask: mask})
	}
	for i := 0; i < 256; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), 0, 0}, Mask: mask})
	}
	mask24 := []byte{255, 255, 255, 0}
	for i := 42; i < 45; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{192, 168, byte(i), 0}, Mask: mask24})
	}
	return pl
}

func initGlobalPredefinedPools() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 256*256)
	mask := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), byte(j), 0}, Mask: mask})
		}
	}
	return pl
}

// Check subnets size. In case configured subnet is v6 and host size is
// greater than 32 bits, adjust subnet to /96.
func adjustAndCheckSubnetSize(subnet *net.IPNet) (*net.IPNet, error) {
	ones, bits := subnet.Mask.Size()
	if v6 == getAddressVersion(subnet.IP) {
		if ones < minNetSizeV6 {
			return nil, ipamapi.ErrInvalidPool
		}
		if ones < minNetSizeV6Eff {
			newMask := net.CIDRMask(minNetSizeV6Eff, bits)
			return &net.IPNet{IP: subnet.IP, Mask: newMask}, nil
		}
	} else {
		if ones < minNetSize {
			return nil, ipamapi.ErrInvalidPool
		}
	}
	return subnet, nil
}

// It generates the ip address in the passed subnet specified by
// the passed host address ordinal
func generateAddress(ordinal uint32, network *net.IPNet) net.IP {
	var address [16]byte

	// Get network portion of IP
	if getAddressVersion(network.IP) == v4 {
		copy(address[:], network.IP.To4())
	} else {
		copy(address[:], network.IP)
	}

	end := len(network.Mask)
	addIntToIP(address[:end], ordinal)

	return net.IP(address[:end])
}

func getAddressVersion(ip net.IP) ipVersion {
	if ip.To4() == nil {
		return v6
	}
	return v4
}

// Adds the ordinal IP to the current array
// 192.168.0.0 + 53 => 192.168.53
func addIntToIP(array []byte, ordinal uint32) {
	for i := len(array) - 1; i >= 0; i-- {
		array[i] |= (byte)(ordinal & 0xff)
		ordinal >>= 8
	}
}

// Convert an ordinal to the respective IP address
func ipToUint32(ip []byte) uint32 {
	value := uint32(0)
	for i := 0; i < len(ip); i++ {
		j := len(ip) - 1 - i
		value += uint32(ip[i]) << uint(j*8)
	}
	return value
}
