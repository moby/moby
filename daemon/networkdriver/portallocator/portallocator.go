package portallocator

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"

	"github.com/docker/docker/daemon/networkdriver/allocator"
)

type protoMap map[string]*allocator.Allocator

func newProtoMap() protoMap {
	return protoMap{
		"tcp": allocator.NewAllocator(big.NewInt(BeginPortRange), big.NewInt(EndPortRange)),
		"udp": allocator.NewAllocator(big.NewInt(BeginPortRange), big.NewInt(EndPortRange)),
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
		if err := mapping.Allocate(portToBigInt(port)); err == nil {
			return port, nil
		}
		return 0, NewErrPortAlreadyAllocated(ipstr, port)
	}

	allocatedPort, err := mapping.AllocateFirstAvailable()
	if err != nil {
		return 0, ErrAllPortsAllocated
	}
	return bigIntToPort(allocatedPort), nil
}

func bigIntToPort(value *big.Int) int {
	return int(value.Int64())
}

func portToBigInt(port int) *big.Int {
	return big.NewInt(int64(port))
}

// ReleasePort releases port from global ports pool for specified ip and proto.
func ReleasePort(ip net.IP, proto string, port int) error {
	mutex.Lock()
	defer mutex.Unlock()

	if ip == nil {
		ip = defaultIP
	}
	if protomap, ok := globalMap[ip.String()]; ok {
		protomap[proto].Release(portToBigInt(port))
	}
	return nil
}

// ReleaseAll releases all ports for all ips.
func ReleaseAll() error {
	mutex.Lock()
	for _, ipmap := range globalMap {
		for _, allocator := range ipmap {
			allocator.ReleaseAll()
		}
	}
	globalMap = ipMapping{}
	mutex.Unlock()
	return nil
}
