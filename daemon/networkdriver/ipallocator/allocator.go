package ipallocator

import (
	"errors"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver"
)

type IpStatus struct {
	active bool
	name   string
	ts     time.Time
}

// allocatedMap is thread-unsafe set of allocated IP
type allocatedMap struct {
	list  map[string]*IpStatus
	cache map[string]net.IP
	last  *big.Int
	begin *big.Int
	end   *big.Int
}

func newAllocatedMap(network *net.IPNet) *allocatedMap {
	firstIP, lastIP := networkdriver.NetworkRange(network)
	begin := big.NewInt(0).Add(ipToBigInt(firstIP), big.NewInt(1))
	end := big.NewInt(0).Sub(ipToBigInt(lastIP), big.NewInt(1))

	return &allocatedMap{
		list:  make(map[string]*IpStatus),
		cache: make(map[string]net.IP),
		begin: begin,
		end:   end,
		last:  big.NewInt(0).Sub(begin, big.NewInt(1)), // so first allocated will be begin
	}
}

type networkSet map[string]*allocatedMap

var (
	ErrNoAvailableIPs           = errors.New("no available ip addresses on network")
	ErrIPAlreadyAllocated       = errors.New("ip already allocated")
	ErrIPOutOfRange             = errors.New("requested ip is out of range")
	ErrNetworkAlreadyRegistered = errors.New("network already registered")
	ErrBadSubnet                = errors.New("network does not contain specified subnet")
)

type IPAllocator struct {
	allocatedIPs networkSet
	mutex        sync.Mutex
}

func New() *IPAllocator {
	return &IPAllocator{networkSet{}, sync.Mutex{}}
}

// RegisterSubnet registers network in global allocator with bounds
// defined by subnet. If you want to use network range you must call
// this method before first RequestIP, otherwise full network range will be used
func (a *IPAllocator) RegisterSubnet(network *net.IPNet, subnet *net.IPNet) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	key := network.String()
	if _, ok := a.allocatedIPs[key]; ok {
		return ErrNetworkAlreadyRegistered
	}
	n := newAllocatedMap(network)
	beginIP, endIP := networkdriver.NetworkRange(subnet)
	begin := big.NewInt(0).Add(ipToBigInt(beginIP), big.NewInt(1))
	end := big.NewInt(0).Sub(ipToBigInt(endIP), big.NewInt(1))

	// Check that subnet is within network
	if !(begin.Cmp(n.begin) >= 0 && end.Cmp(n.end) <= 0 && begin.Cmp(end) == -1) {
		return ErrBadSubnet
	}
	n.begin.Set(begin)
	n.end.Set(end)
	n.last.Sub(begin, big.NewInt(1))
	a.allocatedIPs[key] = n
	return nil
}

// RequestIP requests an available ip from the given network.  It
// will return the next available ip if the ip provided is nil.  If the
// ip provided is not nil it will validate that the provided ip is available
// for use or return an error
func (a *IPAllocator) RequestIP(network *net.IPNet, ip net.IP, name string) (net.IP, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	key := network.String()
	allocated, ok := a.allocatedIPs[key]
	if !ok {
		allocated = newAllocatedMap(network)
		a.allocatedIPs[key] = allocated
	}

	if ip == nil {
		newIp, err := allocated.getNextIP(name)
		if err == nil {
			allocated.markActive(name, newIp)
		}
		return newIp, err

	}
	return allocated.checkIP(ip, name)
}

// ReleaseIP adds the provided ip back into the pool of
// available ips to be returned for use.
func (a *IPAllocator) ReleaseIP(network *net.IPNet, ip net.IP) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if allocated, exists := a.allocatedIPs[network.String()]; exists {
		if ipStatus, ok := allocated.list[ip.String()]; ok {
			ipStatus.active = false
			ipStatus.ts = time.Now()
		}
	}
	return nil
}

func (allocated *allocatedMap) checkIP(ip net.IP, name string) (net.IP, error) {
	if ipStatus, ok := allocated.list[ip.String()]; ok {
		if ipStatus.active {
			return nil, ErrIPAlreadyAllocated
		}
	}

	pos := ipToBigInt(ip)
	// Verify that the IP address is within our network range.
	if pos.Cmp(allocated.begin) == -1 || pos.Cmp(allocated.end) == 1 {
		return nil, ErrIPOutOfRange
	}

	allocated.markActive(name, ip)
	return ip, nil
}

func (allocated *allocatedMap) markActive(name string, ip net.IP) {
	if ipStatus, exists := allocated.list[ip.String()]; exists {
		if name != ipStatus.name {
			// ip is allocated to a new name, delete old cache
			delete(allocated.cache, ipStatus.name)
		}
	}
	allocated.list[ip.String()] = &IpStatus{active: true, name: name, ts: time.Now()}
	allocated.cache[name] = ip
}

// return an available ip if one is currently available.  If not,
// return the next available ip for the network
func (allocated *allocatedMap) getNextIP(name string) (net.IP, error) {
	if cachedIp, cached := allocated.cache[name]; cached {
		ipStatus, hasStatus := allocated.list[cachedIp.String()]
		if hasStatus && (!ipStatus.active) {
			return cachedIp, nil
		}
	}
	recycledIp := new(net.IP)
	recycledTs := time.Now()
	pos := big.NewInt(0).Set(allocated.last)
	allRange := big.NewInt(0).Sub(allocated.end, allocated.begin)
	for i := big.NewInt(0); i.Cmp(allRange) <= 0; i.Add(i, big.NewInt(1)) {
		pos.Add(pos, big.NewInt(1))
		if pos.Cmp(allocated.end) == 1 {
			pos.Set(allocated.begin)
		}
		if ipStatus, ok := allocated.list[bigIntToIP(pos).String()]; ok {
			if (!ipStatus.active) && ipStatus.ts.Before(recycledTs) {
				*recycledIp = bigIntToIP(pos)
				recycledTs = ipStatus.ts
			}
			continue
		}
		allocated.last.Set(pos)
		return bigIntToIP(pos), nil
	}
	if *recycledIp != nil {
		return *recycledIp, nil
	}
	return nil, ErrNoAvailableIPs
}

// Converts a 4 bytes IP into a 128 bit integer
func ipToBigInt(ip net.IP) *big.Int {
	x := big.NewInt(0)
	if ip4 := ip.To4(); ip4 != nil {
		return x.SetBytes(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return x.SetBytes(ip6)
	}

	logrus.Errorf("ipToBigInt: Wrong IP length! %s", ip)
	return nil
}

// Converts 128 bit integer into a 4 bytes IP address
func bigIntToIP(v *big.Int) net.IP {
	return net.IP(v.Bytes())
}
