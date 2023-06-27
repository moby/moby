//go:build linux
// +build linux

package packet

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"os"

	"github.com/josharian/native"
	"github.com/mdlayher/socket"
	"golang.org/x/sys/unix"
)

// A conn is the net.PacketConn implementation for packet sockets. We can use
// socket.Conn directly on Linux to implement most of the necessary methods.
type conn = socket.Conn

// readFrom implements the net.PacketConn ReadFrom method using recvfrom(2).
func (c *Conn) readFrom(b []byte) (int, net.Addr, error) {
	// From net.PacketConn documentation:
	//
	// "[ReadFrom] returns the number of bytes read (0 <= n <= len(p)) and any
	// error encountered. Callers should always process the n > 0 bytes returned
	// before considering the error err."
	//
	// c.opError will return nil if no error, but either way we return all the
	// information that we have.
	n, sa, err := c.c.Recvfrom(context.Background(), b, 0)
	return n, fromSockaddr(sa), c.opError(opRead, err)
}

// writeTo implements the net.PacketConn WriteTo method.
func (c *Conn) writeTo(b []byte, addr net.Addr) (int, error) {
	sa, err := c.toSockaddr("sendto", addr)
	if err != nil {
		return 0, c.opError(opWrite, err)
	}

	// TODO(mdlayher): it's curious that unix.Sendto does not return the number
	// of bytes actually sent. Fake it for now, but investigate upstream.
	if err := c.c.Sendto(context.Background(), b, 0, sa); err != nil {
		return 0, c.opError(opWrite, err)
	}

	return len(b), nil
}

// setPromiscuous wraps setsockopt(2) for the unix.PACKET_MR_PROMISC option.
func (c *Conn) setPromiscuous(enable bool) error {
	mreq := unix.PacketMreq{
		Ifindex: int32(c.ifIndex),
		Type:    unix.PACKET_MR_PROMISC,
	}

	membership := unix.PACKET_DROP_MEMBERSHIP
	if enable {
		membership = unix.PACKET_ADD_MEMBERSHIP
	}

	return c.opError(
		opSetsockopt,
		c.c.SetsockoptPacketMreq(unix.SOL_PACKET, membership, &mreq),
	)
}

// stats wraps getsockopt(2) for tpacket_stats* types.
func (c *Conn) stats() (*Stats, error) {
	const (
		level = unix.SOL_PACKET
		name  = unix.PACKET_STATISTICS
	)

	// Try to fetch V3 statistics first, they contain more detailed information.
	if stats, err := c.c.GetsockoptTpacketStatsV3(level, name); err == nil {
		return &Stats{
			Packets:          stats.Packets,
			Drops:            stats.Drops,
			FreezeQueueCount: stats.Freeze_q_cnt,
		}, nil
	}

	// There was an error fetching v3 stats, try to fall back.
	stats, err := c.c.GetsockoptTpacketStats(level, name)
	if err != nil {
		return nil, c.opError(opGetsockopt, err)
	}

	return &Stats{
		Packets: stats.Packets,
		Drops:   stats.Drops,
		// FreezeQueueCount is not present.
	}, nil
}

// listen is the entry point for Listen on Linux.
func listen(ifi *net.Interface, socketType Type, protocol int, cfg *Config) (*Conn, error) {
	if cfg == nil {
		// Default configuration.
		cfg = &Config{}
	}

	// Convert Type to the matching SOCK_* constant.
	var typ int
	switch socketType {
	case Raw:
		typ = unix.SOCK_RAW
	case Datagram:
		typ = unix.SOCK_DGRAM
	default:
		return nil, errors.New("packet: invalid Type value")
	}

	// Protocol is intentionally zero in call to socket(2); we can set it on
	// bind(2) instead. Package raw notes: "Do not specify a protocol to avoid
	// capturing packets which to not match cfg.Filter."
	c, err := socket.Socket(unix.AF_PACKET, typ, 0, network, nil)
	if err != nil {
		return nil, err
	}

	conn, err := bind(c, ifi.Index, protocol, cfg)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	return conn, nil
}

// bind binds the *socket.Conn to finalize *Conn setup.
func bind(c *socket.Conn, ifIndex, protocol int, cfg *Config) (*Conn, error) {
	if len(cfg.Filter) > 0 {
		// The caller wants to apply a BPF filter before bind(2).
		if err := c.SetBPF(cfg.Filter); err != nil {
			return nil, err
		}
	}

	// packet(7) says we sll_protocol must be in network byte order.
	pnet, err := htons(protocol)
	if err != nil {
		return nil, err
	}

	// TODO(mdlayher): investigate the possibility of sll_ifindex = 0 because we
	// could bind to any interface.
	err = c.Bind(&unix.SockaddrLinklayer{
		Protocol: pnet,
		Ifindex:  ifIndex,
	})
	if err != nil {
		return nil, err
	}

	lsa, err := c.Getsockname()
	if err != nil {
		return nil, err
	}

	// Parse the physical layer address; sll_halen tells us how many bytes of
	// sll_addr we should treat as valid.
	lsall := lsa.(*unix.SockaddrLinklayer)
	addr := make(net.HardwareAddr, lsall.Halen)
	copy(addr, lsall.Addr[:])

	return &Conn{
		c: c,

		addr:     &Addr{HardwareAddr: addr},
		ifIndex:  ifIndex,
		protocol: pnet,
	}, nil
}

// fromSockaddr converts an opaque unix.Sockaddr to *Addr. If sa is nil, it
// returns nil. It panics if sa is not of type *unix.SockaddrLinklayer.
func fromSockaddr(sa unix.Sockaddr) *Addr {
	if sa == nil {
		return nil
	}

	sall := sa.(*unix.SockaddrLinklayer)

	return &Addr{
		// The syscall already allocated sa; just slice into it with the
		// appropriate length and type conversion rather than making a copy.
		HardwareAddr: net.HardwareAddr(sall.Addr[:sall.Halen]),
	}
}

// toSockaddr converts a net.Addr to an opaque unix.Sockaddr. It returns an
// error if the fields cannot be packed into a *unix.SockaddrLinklayer.
func (c *Conn) toSockaddr(
	op string,
	addr net.Addr,
) (unix.Sockaddr, error) {
	// The typical error convention for net.Conn types is
	// net.OpError(os.SyscallError(syscall.Errno)), so all calls here should
	// return os.SyscallError(syscall.Errno) so the caller can apply the final
	// net.OpError wrapper.

	// Ensure the correct Addr type.
	a, ok := addr.(*Addr)
	if !ok || a.HardwareAddr == nil {
		return nil, os.NewSyscallError(op, unix.EINVAL)
	}

	// Pack Addr and Conn metadata into the appropriate sockaddr fields. From
	// packet(7):
	//
	// "When you send packets it is enough to specify sll_family, sll_addr,
	// sll_halen, sll_ifindex, and sll_protocol. The other fields should be 0."
	//
	// sll_family is set on the conversion to unix.RawSockaddrLinklayer.
	sa := unix.SockaddrLinklayer{
		Ifindex:  c.ifIndex,
		Protocol: c.protocol,
	}

	// Ensure the input address does not exceed the amount of space available;
	// for example an IPoIB address is 20 bytes.
	if len(a.HardwareAddr) > len(sa.Addr) {
		return nil, os.NewSyscallError(op, unix.EINVAL)
	}

	sa.Halen = uint8(len(a.HardwareAddr))
	copy(sa.Addr[:], a.HardwareAddr)

	return &sa, nil
}

// htons converts a short (uint16) from host-to-network byte order.
func htons(i int) (uint16, error) {
	if i < 0 || i > math.MaxUint16 {
		return 0, errors.New("packet: protocol value out of range")
	}

	// Store as big endian, retrieve as native endian.
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(i))

	return native.Endian.Uint16(b[:]), nil
}
