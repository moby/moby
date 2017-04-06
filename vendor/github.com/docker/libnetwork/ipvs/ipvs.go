// +build linux

package ipvs

import (
	"net"
	"syscall"

	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

// Service defines an IPVS service in its entirety.
type Service struct {
	// Virtual service address.
	Address  net.IP
	Protocol uint16
	Port     uint16
	FWMark   uint32 // Firewall mark of the service.

	// Virtual service options.
	SchedName     string
	Flags         uint32
	Timeout       uint32
	Netmask       uint32
	AddressFamily uint16
	PEName        string
}

// Destination defines an IPVS destination (real server) in its
// entirety.
type Destination struct {
	Address         net.IP
	Port            uint16
	Weight          int
	ConnectionFlags uint32
	AddressFamily   uint16
	UpperThreshold  uint32
	LowerThreshold  uint32
}

// Handle provides a namespace specific ipvs handle to program ipvs
// rules.
type Handle struct {
	seq  uint32
	sock *nl.NetlinkSocket
}

// New provides a new ipvs handle in the namespace pointed to by the
// passed path. It will return a valid handle or an error in case an
// error occurred while creating the handle.
func New(path string) (*Handle, error) {
	setup()

	n := netns.None()
	if path != "" {
		var err error
		n, err = netns.GetFromPath(path)
		if err != nil {
			return nil, err
		}
	}
	defer n.Close()

	sock, err := nl.GetNetlinkSocketAt(n, netns.None(), syscall.NETLINK_GENERIC)
	if err != nil {
		return nil, err
	}

	return &Handle{sock: sock}, nil
}

// Close closes the ipvs handle. The handle is invalid after Close
// returns.
func (i *Handle) Close() {
	if i.sock != nil {
		i.sock.Close()
	}
}

// NewService creates a new ipvs service in the passed handle.
func (i *Handle) NewService(s *Service) error {
	return i.doCmd(s, nil, ipvsCmdNewService)
}

// IsServicePresent queries for the ipvs service in the passed handle.
func (i *Handle) IsServicePresent(s *Service) bool {
	return nil == i.doCmd(s, nil, ipvsCmdGetService)
}

// UpdateService updates an already existing service in the passed
// handle.
func (i *Handle) UpdateService(s *Service) error {
	return i.doCmd(s, nil, ipvsCmdSetService)
}

// DelService deletes an already existing service in the passed
// handle.
func (i *Handle) DelService(s *Service) error {
	return i.doCmd(s, nil, ipvsCmdDelService)
}

// NewDestination creates a new real server in the passed ipvs
// service which should already be existing in the passed handle.
func (i *Handle) NewDestination(s *Service, d *Destination) error {
	return i.doCmd(s, d, ipvsCmdNewDest)
}

// UpdateDestination updates an already existing real server in the
// passed ipvs service in the passed handle.
func (i *Handle) UpdateDestination(s *Service, d *Destination) error {
	return i.doCmd(s, d, ipvsCmdSetDest)
}

// DelDestination deletes an already existing real server in the
// passed ipvs service in the passed handle.
func (i *Handle) DelDestination(s *Service, d *Destination) error {
	return i.doCmd(s, d, ipvsCmdDelDest)
}
