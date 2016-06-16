package netlink

import (
	"sync/atomic"
	"syscall"

	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

// Empty handle used by the netlink package methods
var pkgHandle = &Handle{}

// Handle is an handle for the netlink requests
// on a specific network namespace. All the requests
// share the same netlink socket, which gets released
// when the handle is deleted.
type Handle struct {
	seq          uint32
	routeSocket  *nl.NetlinkSocket
	xfrmSocket   *nl.NetlinkSocket
	lookupByDump bool
}

// NewHandle returns a netlink handle on the current network namespace.
func NewHandle() (*Handle, error) {
	return newHandle(netns.None(), netns.None())
}

// NewHandle returns a netlink handle on the network namespace
// specified by ns. If ns=netns.None(), current network namespace
// will be assumed
func NewHandleAt(ns netns.NsHandle) (*Handle, error) {
	return newHandle(ns, netns.None())
}

// NewHandleAtFrom works as NewHandle but allows client to specify the
// new and the origin netns Handle.
func NewHandleAtFrom(newNs, curNs netns.NsHandle) (*Handle, error) {
	return newHandle(newNs, curNs)
}

func newHandle(newNs, curNs netns.NsHandle) (*Handle, error) {
	var (
		err     error
		rSocket *nl.NetlinkSocket
		xSocket *nl.NetlinkSocket
	)
	rSocket, err = nl.GetNetlinkSocketAt(newNs, curNs, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	xSocket, err = nl.GetNetlinkSocketAt(newNs, curNs, syscall.NETLINK_XFRM)
	if err != nil {
		return nil, err
	}
	return &Handle{routeSocket: rSocket, xfrmSocket: xSocket}, nil
}

// Delete releases the resources allocated to this handle
func (h *Handle) Delete() {
	if h.routeSocket != nil {
		h.routeSocket.Close()
	}
	if h.xfrmSocket != nil {
		h.xfrmSocket.Close()
	}
	h.routeSocket, h.xfrmSocket = nil, nil
}

func (h *Handle) newNetlinkRequest(proto, flags int) *nl.NetlinkRequest {
	// Do this so that package API still use nl package variable nextSeqNr
	if h.routeSocket == nil {
		return nl.NewNetlinkRequest(proto, flags)
	}
	return &nl.NetlinkRequest{
		NlMsghdr: syscall.NlMsghdr{
			Len:   uint32(syscall.SizeofNlMsghdr),
			Type:  uint16(proto),
			Flags: syscall.NLM_F_REQUEST | uint16(flags),
			Seq:   atomic.AddUint32(&h.seq, 1),
		},
		RouteSocket: h.routeSocket,
		XfmrSocket:  h.xfrmSocket,
	}
}
