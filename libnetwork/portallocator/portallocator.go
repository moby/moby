package portallocator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"

	"github.com/containerd/log"
)

type ipMapping map[string]protoMap

var (
	// ErrAllPortsAllocated is returned when no more ports are available
	ErrAllPortsAllocated = errors.New("all ports are allocated")
	// ErrUnknownProtocol is returned when an unknown protocol was specified
	ErrUnknownProtocol = errors.New("unknown protocol")
	defaultIP          = net.ParseIP("0.0.0.0")
	once               sync.Once
	instance           *PortAllocator
)

// ErrPortAlreadyAllocated is the returned error information when a requested port is already being used
type ErrPortAlreadyAllocated struct {
	ip   string
	port int
}

func newErrPortAlreadyAllocated(ip string, port int) ErrPortAlreadyAllocated {
	return ErrPortAlreadyAllocated{
		ip:   ip,
		port: port,
	}
}

// IP returns the address to which the used port is associated
func (e ErrPortAlreadyAllocated) IP() string {
	return e.ip
}

// Port returns the value of the already used port
func (e ErrPortAlreadyAllocated) Port() int {
	return e.port
}

// IPPort returns the address and the port in the form ip:port
func (e ErrPortAlreadyAllocated) IPPort() string {
	return fmt.Sprintf("%s:%d", e.ip, e.port)
}

// Error is the implementation of error.Error interface
func (e ErrPortAlreadyAllocated) Error() string {
	return fmt.Sprintf("Bind for %s:%d failed: port is already allocated", e.ip, e.port)
}

type (
	// PortAllocator manages the transport ports database
	PortAllocator struct {
		mutex sync.Mutex
		ipMap ipMapping
		Begin int
		End   int
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

// Get returns the PortAllocator
func Get() *PortAllocator {
	// Port Allocator is a singleton
	once.Do(func() {
		instance = newInstance()
	})
	return instance
}

func newInstance() *PortAllocator {
	start, end, err := getDynamicPortRange()
	if err != nil {
		log.G(context.TODO()).WithError(err).Infof("falling back to default port range %d-%d", defaultPortRangeStart, defaultPortRangeEnd)
		start, end = defaultPortRangeStart, defaultPortRangeEnd
	}
	return &PortAllocator{
		ipMap: ipMapping{},
		Begin: start,
		End:   end,
	}
}

// RequestPort requests new port from global ports pool for specified ip and proto.
// If port is 0 it returns first free port. Otherwise it checks port availability
// in proto's pool and returns that port or error if port is already busy.
func (p *PortAllocator) RequestPort(ip net.IP, proto string, port int) (int, error) {
	return p.RequestPortInRange(ip, proto, port, port)
}

// RequestPortInRange is equivalent to [RequestPortsInRange] with a single IP address.
//
// If ip is nil, a port is instead requested for the defaultIP.
func (p *PortAllocator) RequestPortInRange(ip net.IP, proto string, portStart, portEnd int) (int, error) {
	if ip == nil {
		ip = defaultIP
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
		return 0, ErrUnknownProtocol
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	pMaps := make([]*portMap, len(ips))
	for i, ip := range ips {
		ipstr := ip.String()
		if _, ok := p.ipMap[ipstr]; !ok {
			p.ipMap[ipstr] = protoMap{
				"tcp":  p.newPortMap(),
				"udp":  p.newPortMap(),
				"sctp": p.newPortMap(),
			}
		}
		pMaps[i] = p.ipMap[ipstr][proto]
	}

	// Handle a request for a specific port.
	if portStart > 0 && portStart == portEnd {
		for i, pMap := range pMaps {
			if _, allocated := pMap.p[portStart]; allocated {
				return 0, newErrPortAlreadyAllocated(ips[i].String(), portStart)
			}
		}
		for _, pMap := range pMaps {
			pMap.p[portStart] = struct{}{}
		}
		return portStart, nil
	}

	// Handle a request for a port range.

	// Create/fetch ranges for each portMap.
	pRanges := make([]*portRange, len(pMaps))
	for i, pMap := range pMaps {
		var err error
		pRanges[i], err = pMap.getPortRange(portStart, portEnd)
		if err != nil {
			return 0, err
		}
	}

	// Starting after the last port allocated for the first address, search
	// for a port that's available in all ranges.
	port := pRanges[0].last
	for i := pRanges[0].begin; i <= pRanges[0].end; i++ {
		port++
		if port > pRanges[0].end {
			port = pRanges[0].begin
		}

		if !slices.ContainsFunc(pMaps, func(pMap *portMap) bool {
			_, allocated := pMap.p[port]
			return allocated
		}) {
			for pi, pMap := range pMaps {
				pMap.p[port] = struct{}{}
				pRanges[pi].last = port
			}
			return port, nil
		}
	}
	return 0, ErrAllPortsAllocated
}

// ReleasePort releases port from global ports pool for specified ip and proto.
func (p *PortAllocator) ReleasePort(ip net.IP, proto string, port int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if ip == nil {
		ip = defaultIP
	}
	protomap, ok := p.ipMap[ip.String()]
	if !ok {
		return
	}
	delete(protomap[proto].p, port)
}

func (p *PortAllocator) newPortMap() *portMap {
	defaultKey := getRangeKey(p.Begin, p.End)
	pm := &portMap{
		p:            map[int]struct{}{},
		defaultRange: defaultKey,
		portRanges: map[string]*portRange{
			defaultKey: newPortRange(p.Begin, p.End),
		},
	}
	return pm
}

// ReleaseAll releases all ports for all ips.
func (p *PortAllocator) ReleaseAll() error {
	p.mutex.Lock()
	p.ipMap = ipMapping{}
	p.mutex.Unlock()
	return nil
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

func (pm *portMap) getPortRange(portStart, portEnd int) (*portRange, error) {
	var key string
	if portStart == 0 && portEnd == 0 {
		key = pm.defaultRange
	} else {
		key = getRangeKey(portStart, portEnd)
		if portStart == portEnd ||
			portStart == 0 || portEnd == 0 ||
			portEnd < portStart {
			return nil, fmt.Errorf("invalid port range: %s", key)
		}
	}

	// Return existing port range, if already known.
	if pr, exists := pm.portRanges[key]; exists {
		return pr, nil
	}

	// Otherwise create a new port range.
	pr := newPortRange(portStart, portEnd)
	pm.portRanges[key] = pr
	return pr, nil
}
