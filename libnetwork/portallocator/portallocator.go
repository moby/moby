package portallocator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/containerd/log"
)

type ipMapping map[netip.Addr]protoMap

var (
	// errAllPortsAllocated is returned when no more ports are available
	errAllPortsAllocated = errors.New("all ports are allocated")
	// errUnknownProtocol is returned when an unknown protocol was specified
	errUnknownProtocol = errors.New("unknown protocol")
	once               sync.Once
	instance           *PortAllocator
)

// alreadyAllocatedErr is the returned error information when a requested port is already being used
type alreadyAllocatedErr struct {
	ip   string
	port int
}

// Error is the implementation of error.Error interface
func (e alreadyAllocatedErr) Error() string {
	return fmt.Sprintf("Bind for %s:%d failed: port is already allocated", e.ip, e.port)
}

type (
	// PortAllocator manages the transport ports database
	PortAllocator struct {
		mutex     sync.Mutex
		defaultIP net.IP
		ipMap     ipMapping
		begin     int
		end       int
	}
	portRange struct {
		begin int
		end   int
		last  int
	}
	portMap struct {
		p            map[int]struct{}
		defaultRange string
		portRanges   map[string]*portRange
	}
	protoMap map[string]*portMap
)

// GetPortRange returns the PortAllocator's default port range.
//
// This function is for internal use in tests, and must not be used
// for other purposes.
func GetPortRange() (start, end uint16) {
	p := Get()
	return uint16(p.begin), uint16(p.end)
}

// Get returns the PortAllocator
func Get() *PortAllocator {
	// Port Allocator is a singleton
	once.Do(func() {
		instance = newInstance()
	})
	return instance
}

func newInstance() *PortAllocator {
	begin, end := dynamicPortRange()
	return &PortAllocator{
		ipMap:     makeIpMapping(begin, end),
		defaultIP: net.IPv4zero,
		begin:     begin,
		end:       end,
	}
}

func dynamicPortRange() (start, end int) {
	begin, end, err := getDynamicPortRange()
	if err != nil {
		log.G(context.TODO()).WithError(err).Infof("falling back to default port range %d-%d", defaultPortRangeStart, defaultPortRangeEnd)
		return defaultPortRangeStart, defaultPortRangeEnd
	}
	return begin, end
}

func makeIpMapping(begin, end int) ipMapping {
	return ipMapping{netip.IPv4Unspecified(): makeProtoMap(begin, end)}
}

func makeProtoMap(begin, end int) protoMap {
	return protoMap{
		"tcp":  newPortMap(begin, end),
		"udp":  newPortMap(begin, end),
		"sctp": newPortMap(begin, end),
	}
}

// RequestPort requests new port from global ports pool for specified ip and proto.
// If port is 0 it returns first free port. Otherwise it checks port availability
// in proto's pool and returns that port or error if port is already busy.
func (p *PortAllocator) RequestPort(ip net.IP, proto string, port int) (int, error) {
	if ip == nil {
		ip = p.defaultIP // FIXME(thaJeztah): consider making this a required argument and producing an error instead, or set default when constructing.
	}
	return p.RequestPortsInRange([]net.IP{ip}, proto, port, port)
}

// RequestPortInRange is equivalent to [PortAllocator.RequestPortsInRange] with
// a single IP address. If ip is nil, a port is instead requested for the
// default IP (0.0.0.0).
func (p *PortAllocator) RequestPortInRange(ip net.IP, proto string, portStart, portEnd int) (int, error) {
	if ip == nil {
		ip = p.defaultIP // FIXME(thaJeztah): consider making this a required argument and producing an error instead, or set default when constructing.
	}
	return p.RequestPortsInRange([]net.IP{ip}, proto, portStart, portEnd)
}

// RequestPortsInRange requests new ports from the global ports pool, for proto and each of ips.
// If portStart and portEnd are 0 it returns the first free port in the default ephemeral range.
// If portStart != portEnd it returns the first free port in the requested range.
// Otherwise, (portStart == portEnd) it checks port availability in the requested proto's port-pool
// and returns that port or error if port is already busy.
func (p *PortAllocator) RequestPortsInRange(ips []net.IP, proto string, portStart, portEnd int) (int, error) {
	if proto != "tcp" && proto != "udp" && proto != "sctp" {
		return 0, errUnknownProtocol
	}
	if portStart != 0 || portEnd != 0 {
		// Validate custom port-range
		if portStart == 0 || portEnd == 0 || portEnd < portStart {
			return 0, fmt.Errorf("invalid port range: %d-%d", portStart, portEnd)
		}
	}
	if len(ips) == 0 {
		return 0, fmt.Errorf("no IP addresses specified")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Collect the portMap for the required proto and each of the IP addresses.
	// If there's a new IP address, create portMap objects for each of the protocols
	// and collect the one that's needed for this request.
	// Mark these portMap objects as needing port allocations.
	type portMapRef struct {
		portMap  *portMap
		allocate bool
	}
	pMaps := map[netip.Addr]*portMapRef{}
	ipv4Unspec := netip.IPv4Unspecified()
	ipv4UnspecInIPs := false
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return 0, fmt.Errorf("invalid IP address: %s", ip)
		}
		addr = addr.Unmap()
		// Make sure a protoMap exists for each ip address.
		if _, ok := p.ipMap[addr]; !ok {
			p.ipMap[addr] = makeProtoMap(p.begin, p.end)
		}
		// Remember the protoMap[proto] portMap, it needs the port allocation.
		pMaps[addr] = &portMapRef{
			portMap:  p.ipMap[addr][proto],
			allocate: true,
		}
		// Remember whether 0.0.0.0 is in ips.
		ipv4UnspecInIPs = ipv4UnspecInIPs || addr == ipv4Unspec
	}

	// If ips includes 0.0.0.0, the port needs to be free in all ipMaps.
	// Otherwise, the port needs to be free in the per-address maps and 0.0.0.0.
	//
	// Collect the additional portMaps where the port needs to be free, but
	// don't mark them as needing port allocation.
	if ipv4UnspecInIPs {
		for addr, ipm := range p.ipMap {
			if _, ok := pMaps[addr]; !ok {
				pMaps[addr] = &portMapRef{portMap: ipm[proto]}
			}
		}
	} else {
		pMaps[ipv4Unspec] = &portMapRef{portMap: p.ipMap[ipv4Unspec][proto]}
	}

	// Handle a request for a specific port.
	if portStart > 0 && portStart == portEnd {
		for addr, pMap := range pMaps {
			if _, allocated := pMap.portMap.p[portStart]; allocated {
				return 0, alreadyAllocatedErr{ip: addr.String(), port: portStart}
			}
		}
		for _, pMap := range pMaps {
			if pMap.allocate {
				pMap.portMap.p[portStart] = struct{}{}
			}
		}
		return portStart, nil
	}

	// Handle a request for a port range.

	// Create/fetch ranges for each portMap.
	pRanges := map[netip.Addr]*portRange{}
	for addr, pMap := range pMaps {
		pRanges[addr] = pMap.portMap.getPortRange(portStart, portEnd)
	}

	// Arbitrarily starting after the last port allocated for the first address, search
	// for a port that's available in all ranges.
	firstAddr, _ := netip.AddrFromSlice(ips[0])
	firstRange := pRanges[firstAddr.Unmap()]
	port := firstRange.last
	for i := firstRange.begin; i <= firstRange.end; i++ {
		port++
		if port > firstRange.end {
			port = firstRange.begin
		}

		portAlreadyAllocated := func() bool {
			for _, pMap := range pMaps {
				if _, ok := pMap.portMap.p[port]; ok {
					return true
				}
			}
			return false
		}

		if !portAlreadyAllocated() {
			for addr, pMap := range pMaps {
				if pMap.allocate {
					pMap.portMap.p[port] = struct{}{}
					pRanges[addr].last = port
				}
			}
			return port, nil
		}
	}
	return 0, errAllPortsAllocated
}

// ReleasePort releases port from global ports pool for specified ip and proto.
func (p *PortAllocator) ReleasePort(ip net.IP, proto string, port int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if ip == nil {
		ip = p.defaultIP // FIXME(thaJeztah): consider making this a required argument and producing an error instead, or set default when constructing.
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return
	}
	protomap, ok := p.ipMap[addr.Unmap()]
	if !ok {
		return
	}
	delete(protomap[proto].p, port)
}

// ReleaseAll releases all ports for all ips.
func (p *PortAllocator) ReleaseAll() {
	begin, end := dynamicPortRange()
	p.mutex.Lock()
	p.ipMap = makeIpMapping(begin, end)
	p.mutex.Unlock()
}

func getRangeKey(portStart, portEnd int) string {
	return fmt.Sprintf("%d-%d", portStart, portEnd)
}

func newPortRange(portStart, portEnd int) *portRange {
	return &portRange{
		begin: portStart,
		end:   portEnd,
		last:  portEnd,
	}
}

func newPortMap(portStart, portEnd int) *portMap {
	defaultKey := getRangeKey(portStart, portEnd)
	return &portMap{
		p:            map[int]struct{}{},
		defaultRange: defaultKey,
		portRanges: map[string]*portRange{
			defaultKey: newPortRange(portStart, portEnd),
		},
	}
}

func (pm *portMap) getPortRange(portStart, portEnd int) *portRange {
	var key string
	if portStart == 0 && portEnd == 0 {
		key = pm.defaultRange
	} else {
		key = getRangeKey(portStart, portEnd)
	}

	// Return existing port range, if already known.
	if pr, exists := pm.portRanges[key]; exists {
		return pr
	}

	// Otherwise create a new port range.
	pr := newPortRange(portStart, portEnd)
	pm.portRanges[key] = pr
	return pr
}
