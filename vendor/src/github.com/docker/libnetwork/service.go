package libnetwork

import (
	"net"
	"sync"
)

var (
	// A global monotonic counter to assign firewall marks to
	// services.
	fwMarkCtr   uint32 = 256
	fwMarkCtrMu sync.Mutex
)

type service struct {
	name string // Service Name
	id   string // Service ID

	// Map of loadbalancers for the service one-per attached
	// network. It is keyed with network ID.
	loadBalancers map[string]*loadBalancer

	// List of ingress ports exposed by the service
	ingressPorts []*PortConfig

	sync.Mutex
}

type loadBalancer struct {
	vip    net.IP
	fwMark uint32

	// Map of backend IPs backing this loadbalancer on this
	// network. It is keyed with endpoint ID.
	backEnds map[string]net.IP

	// Back pointer to service to which the loadbalancer belongs.
	service *service
}
