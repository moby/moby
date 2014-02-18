package networkdriver

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dotcloud/docker/pkg/netlink"
)

var (
	networkGetRoutesFct = netlink.NetworkGetRoutes
	ErrNoDefaultRoute   = errors.New("no default route")
)

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

func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := networkGetRoutesFct()
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.IPNet != nil && NetworkOverlaps(toCheck, network.IPNet) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// Detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

// Calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	var (
		netIP = network.IP
		firstIP, lastIP net.IP
	)

	firstIP = netIP.Mask(network.Mask)
	if tempIP := netIP.To4(); tempIP == nil {
		// Looks weird, but net.IPv4 still returns a 16 byte slice
		lastIP = net.IPv4(0, 0, 0, 0)
	} else {
		netIP = tempIP
		lastIP = net.IPv4(0, 0, 0, 0).To4()
	}

	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

// Given a netmask, calculates the number of available hosts
func NetworkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}
	return int32(binary.BigEndian.Uint32(m)) + 1
}

// Given a netmask, calculates the number of available hosts
func NetworkSize6(mask net.IPMask) (uint64, uint64) {
	m := net.IPMask([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	for i := 0; i < net.IPv6len; i++ {
		m[i] = ^mask[i]
	}

	m1 := make([]byte, 8)
	m2 := make([]byte, 8)
	for i := 0; i < len(m1); i++ {
		n := i + 8
		m1[i] = m[i]
		m2[i] = m[n]
	}
	return binary.BigEndian.Uint64(m1), binary.BigEndian.Uint64(m2)
}

// Return the IPv4 address of a network interface
func GetIfaceAddr(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs4 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); len(ip4) == net.IPv4len {
			addrs4 = append(addrs4, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], nil
}

// Return the IPv6 address of a network interface
func GetIfaceAddr6(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs6 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); ip4 == nil {
			addrs6 = append(addrs6, addr)
		}
	}
	switch {
	case len(addrs6) == 0:
		return nil, fmt.Errorf("Interface %v has no IPv6 addresses", name)
	case len(addrs6) == 1:
		return nil, fmt.Errorf("Interface %v only has a link-local IPv6 address", name)
	case len(addrs6) > 2:
		fmt.Printf("Interface %v has more than 2 IPv6 addresses. Defaulting to using %v\n",
			name, (addrs6[0].(*net.IPNet)).IP)
	}
	return addrs6[0], nil
}

func GetDefaultRouteIface() (*net.Interface, error) {
	rs, err := networkGetRoutesFct()
	if err != nil {
		return nil, fmt.Errorf("unable to get routes: %v", err)
	}
	for _, r := range rs {
		if r.Default {
			return r.Iface, nil
		}
	}
	return nil, ErrNoDefaultRoute
}

// timeNTP() retrieves the current time from NTP's official server,
// returning uint64-encoded 'seconds' and 'fractions' See RFC 5905
func timeNTP() (uint64, uint64, error) {
	ntps, err := net.ResolveUDPAddr("udp", "0.pool.ntp.org:123")
	if err != nil {
		fmt.Errorf("Unable to get NTP time: %v", err)
		return 0, 0, err
	}

	data := make([]byte, 48)
	data[0] = 3<<3 | 3

	con, err := net.DialUDP("udp", nil, ntps)
	defer con.Close()

	_, err = con.Write(data)

	con.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = con.Read(data)
	if err != nil {
		return 0, 0, err
	}

	var sec, frac uint64

	sec = uint64(data[43]) | uint64(data[42])<<8 | uint64(data[41])<<16 | uint64(data[40])<<24
	frac = uint64(data[47]) | uint64(data[46])<<8 | uint64(data[45])<<16 | uint64(data[44])<<24
	return sec, frac, nil
}

// Uint48([]byte) encodes a 48-bit (6 byte) []byte such as an interface MAC address into a uint64.
func Uint48(b []byte) uint64 {
	return uint64(b[5]) | uint64(b[4])<<8 | uint64(b[3])<<16 | uint64(b[2])<<24 | uint64(b[1])<<32 | uint64(b[0])<<40
}

// findMAC() discovers the 'best' interface to use for IPv6 ULA
// generation; it loops through each available interface, looking for a
// non-zero, non-one MAC address.
//
// If none are found, it returns 0.
func findMAC() uint64 {
	interfaces, _ := net.Interfaces()
	for i := range interfaces {
		mac := interfaces[i].HardwareAddr
		if mac != nil {
			macint := Uint48(mac)
			if macint > 1 {
				return macint
			}
		}
	}
	return 0
}

// GenULA() generates Unique Local Addresses for IPv6, implementing the
// algorithm suggested in RFC 4193
func GenULA() net.IPNet {
	ntpsec, ntpfrac, _ := timeNTP()
	mac := findMAC()
	if mac == 0 {
		mac = uint64(123456789123) // non-standard-compliant placeholder in case of error
	}
	key := ntpsec + ntpfrac + uint64(mac)
	keyb := make([]byte, 8)
	binary.BigEndian.PutUint64(keyb, key)
	sha := sha1.New()
	shakey := sha.Sum(keyb)
	ip := net.IP(make([]byte, 16))
	pre := []byte{252}

	for i := 0; i < len(pre); i++ {
		ip[i] = pre[i]
	}

	for i := 0; i < 7; i++ {
		n := i + 1
		ip[n] = shakey[i]
	}

	return net.IPNet{
		IP:	ip,
		Mask:	net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
	}
}

func GenerateIPv6AddressPool() []string {
	var tempIP net.IP
	addresses := make([]string, 4)

	for i := 0; i < len(addresses); i++ {
		tempIP = GenULA().IP
		// Add one for our Gateway as GenULA generates network addresses
		tempIP[15] = tempIP[15] + 1
		addresses[i] = tempIP.String() + "/64"
	}

	return addresses
}

func IsIPv6(ip *net.IP) bool {
	if ip.To4() == nil {
		return true
	}

	return false
}
