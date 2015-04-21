package portallocator

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
)

const (
	DefaultPortRangeStart = 49153
	DefaultPortRangeEnd   = 65535
)

type ipMapping map[string]protoMap

var (
	ErrAllPortsAllocated = errors.New("all ports are allocated")
	ErrUnknownProtocol   = errors.New("unknown protocol")
	defaultIP            = net.ParseIP("0.0.0.0")
)

type ErrPortAlreadyAllocated struct {
	ip   string
	port int
}

func NewErrPortAlreadyAllocated(ip string, port int) ErrPortAlreadyAllocated {
	return ErrPortAlreadyAllocated{
		ip:   ip,
		port: port,
	}
}

func (e ErrPortAlreadyAllocated) IP() string {
	return e.ip
}

func (e ErrPortAlreadyAllocated) Port() int {
	return e.port
}

func (e ErrPortAlreadyAllocated) IPPort() string {
	return fmt.Sprintf("%s:%d", e.ip, e.port)
}

func (e ErrPortAlreadyAllocated) Error() string {
	return fmt.Sprintf("Bind for %s:%d failed: port is already allocated", e.ip, e.port)
}

type (
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

func New() *PortAllocator {
	start, end, err := getDynamicPortRange()
	if err != nil {
		logrus.Warn(err)
		start, end = DefaultPortRangeStart, DefaultPortRangeEnd
	}
	return &PortAllocator{
		ipMap: ipMapping{},
		Begin: start,
		End:   end,
	}
}

func getDynamicPortRange() (start int, end int, err error) {
	const portRangeKernelParam = "/proc/sys/net/ipv4/ip_local_port_range"
	portRangeFallback := fmt.Sprintf("using fallback port range %d-%d", DefaultPortRangeStart, DefaultPortRangeEnd)
	file, err := os.Open(portRangeKernelParam)
	if err != nil {
		return 0, 0, fmt.Errorf("port allocator - %s due to error: %v", portRangeFallback, err)
	}
	n, err := fmt.Fscanf(bufio.NewReader(file), "%d\t%d", &start, &end)
	if n != 2 || err != nil {
		if err == nil {
			err = fmt.Errorf("unexpected count of parsed numbers (%d)", n)
		}
		return 0, 0, fmt.Errorf("port allocator - failed to parse system ephemeral port range from %s - %s: %v", portRangeKernelParam, portRangeFallback, err)
	}
	return start, end, nil
}

// RequestPort requests new port from global or specified ports pool for specified ip and proto.
// If port is 0 it returns first free port. Otherwise it checks port availability
// in pool and returns: a) that port, if available; b) the next available port if busy but range was specified; or
// c) error if no range and port is already busy.
func (p *PortAllocator) RequestPort(ip net.IP, proto string, port int, start int, end int) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if proto != "tcp" && proto != "udp" {
		return 0, ErrUnknownProtocol
	}

	if ip == nil {
		ip = defaultIP
	}
	ipstr := ip.String()
	protomap, ok := p.ipMap[ipstr]
	if !ok {
		protomap = protoMap{
			"tcp": p.newPortMap(),
			"udp": p.newPortMap(),
		}

		p.ipMap[ipstr] = protomap
	}
	mapping := protomap[proto]
	if port > 0 {
		if _, ok := mapping.p[port]; !ok {
			mapping.p[port] = struct{}{}
			return port, nil
		}
		// If we are in a custom range, we can try to auto-allocate again
		if start != 0 && port >= start && port <= end {
			warn := fmt.Sprintf("Port %d is busy, re-allocating from specified range: %d-%d", port, start, end)
			logrus.Warn(warn)
		} else {
			return 0, NewErrPortAlreadyAllocated(ipstr, port)
		}
	}

	port, err := mapping.findPort(start, end)
	if err != nil {
		return 0, err
	}
	return port, nil
}

// ReleasePort releases port from global ports pool for specified ip and proto.
func (p *PortAllocator) ReleasePort(ip net.IP, proto string, port int) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if ip == nil {
		ip = defaultIP
	}
	protomap, ok := p.ipMap[ip.String()]
	if !ok {
		return nil
	}
	delete(protomap[proto].p, port)
	return nil
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
		if portStart == 0 || portEnd == 0 || portEnd < portStart {
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

func (pm *portMap) findPort(start int, end int) (int, error) {
	pr, err := pm.getPortRange(start, end)
	if err != nil {
		return 0, err
	}
	port := pr.last

	for i := 0; i <= pr.end-pr.begin; i++ {
		port++
		if port > pr.end {
			port = pr.begin
		}

		if _, ok := pm.p[port]; !ok {
			pm.p[port] = struct{}{}
			pr.last = port
			return port, nil
		}
	}
	return 0, ErrAllPortsAllocated
}
