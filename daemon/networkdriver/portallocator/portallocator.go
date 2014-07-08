package portallocator

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type portMap struct {
	p    map[int]struct{}
	last int
}

func newPortMap() *portMap {
	return &portMap{
		p: map[int]struct{}{},
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

const (
	BeginPortRange = 49153
	EndPortRange   = 65535
)

var (
	ErrAllPortsAllocated = errors.New("all ports are allocated")
	ErrUnknownProtocol   = errors.New("unknown protocol")
)

var (
	mutex sync.Mutex

	defaultIP = net.ParseIP("0.0.0.0")
	globalMap = ipMapping{}
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

// RequestPort requests new port from global ports pool for specified ip and proto.
// If port is 0 it returns first free port. Otherwise it cheks port availability
// in pool and return that port or error if port is already busy.
func RequestPort(ip net.IP, proto string, port int) (int, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if proto != "tcp" && proto != "udp" {
		return 0, ErrUnknownProtocol
	}

	if ip == nil {
		ip = defaultIP
	}
	ipstr := ip.String()
	protomap, ok := globalMap[ipstr]
	if !ok {
		protomap = newProtoMap()
		globalMap[ipstr] = protomap
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
func ReleasePort(ip net.IP, proto string, port int) error {
	mutex.Lock()
	defer mutex.Unlock()

	if ip == nil {
		ip = defaultIP
	}
	protomap, ok := globalMap[ip.String()]
	if !ok {
		return nil
	}
	delete(protomap[proto].p, port)
	return nil
}

// ReleaseAll releases all ports for all ips.
func ReleaseAll() error {
	mutex.Lock()
	globalMap = ipMapping{}
	mutex.Unlock()
	return nil
}

func (pm *portMap) findPort() (int, error) {
	if pm.last == 0 {
		pm.p[BeginPortRange] = struct{}{}
		pm.last = BeginPortRange
		return BeginPortRange, nil
	}

	for port := pm.last + 1; port != pm.last; port++ {
		if port > EndPortRange {
			port = BeginPortRange
		}

		if _, ok := pm.p[port]; !ok {
			pm.p[port] = struct{}{}
			pm.last = port
			return port, nil
		}
	}
	return 0, ErrAllPortsAllocated
}
