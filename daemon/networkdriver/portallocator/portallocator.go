package portallocator

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	log "github.com/Sirupsen/logrus"
)

const (
	DefaultPortRangeStart = 49153
	DefaultPortRangeEnd   = 65535
)

var (
	beginPortRange = DefaultPortRangeStart
	endPortRange   = DefaultPortRangeEnd
)

type portMap struct {
	p    map[int]struct{}
	last int
}

func newPortMap() *portMap {
	return &portMap{
		p:    map[int]struct{}{},
		last: endPortRange,
	}
}

type protoMap map[string]*portMap

func newProtoMap() protoMap {
	return protoMap{
		"tcp": newPortMap(),
		"udp": newPortMap(),
	}
}

type ipMapping map[string]protoMap

var (
	ErrAllPortsAllocated = errors.New("all ports are allocated")
	ErrUnknownProtocol   = errors.New("unknown protocol")
)

var (
	defaultIP = net.ParseIP("0.0.0.0")

	DefaultPortAllocator = New()
	RequestPort          = DefaultPortAllocator.RequestPort
	ReleasePort          = DefaultPortAllocator.ReleasePort
	ReleaseAll           = DefaultPortAllocator.ReleaseAll
)

type PortAllocator struct {
	mutex sync.Mutex
	ipMap ipMapping
}

func New() *PortAllocator {
	return &PortAllocator{
		ipMap: ipMapping{},
	}
}

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

func init() {
	const portRangeKernelParam = "/proc/sys/net/ipv4/ip_local_port_range"
	portRangeFallback := fmt.Sprintf("using fallback port range %d-%d", beginPortRange, endPortRange)

	file, err := os.Open(portRangeKernelParam)
	if err != nil {
		log.Warnf("port allocator - %s due to error: %v", portRangeFallback, err)
		return
	}
	var start, end int
	n, err := fmt.Fscanf(bufio.NewReader(file), "%d\t%d", &start, &end)
	if n != 2 || err != nil {
		if err == nil {
			err = fmt.Errorf("unexpected count of parsed numbers (%d)", n)
		}
		log.Errorf("port allocator - failed to parse system ephemeral port range from %s - %s: %v", portRangeKernelParam, portRangeFallback, err)
		return
	}
	beginPortRange = start
	endPortRange = end
}

func PortRange() (int, int) {
	return beginPortRange, endPortRange
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

// RequestPort requests new port from global ports pool for specified ip and proto.
// If port is 0 it returns first free port. Otherwise it cheks port availability
// in pool and return that port or error if port is already busy.
func (p *PortAllocator) RequestPort(ip net.IP, proto string, port int) (int, error) {
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
		protomap = newProtoMap()
		p.ipMap[ipstr] = protomap
	}
	mapping := protomap[proto]
	if port > 0 {
		if _, ok := mapping.p[port]; !ok {
			mapping.p[port] = struct{}{}
			return port, nil
		}
		return 0, NewErrPortAlreadyAllocated(ipstr, port)
	}

	port, err := mapping.findPort()
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

// ReleaseAll releases all ports for all ips.
func (p *PortAllocator) ReleaseAll() error {
	p.mutex.Lock()
	p.ipMap = ipMapping{}
	p.mutex.Unlock()
	return nil
}

func (pm *portMap) findPort() (int, error) {
	port := pm.last
	for i := 0; i <= endPortRange-beginPortRange; i++ {
		port++
		if port > endPortRange {
			port = beginPortRange
		}

		if _, ok := pm.p[port]; !ok {
			pm.p[port] = struct{}{}
			pm.last = port
			return port, nil
		}
	}
	return 0, ErrAllPortsAllocated
}
