package netlink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"

	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// RtAttr is shared so it is in netlink_linux.go

const (
	SCOPE_UNIVERSE Scope = unix.RT_SCOPE_UNIVERSE
	SCOPE_SITE     Scope = unix.RT_SCOPE_SITE
	SCOPE_LINK     Scope = unix.RT_SCOPE_LINK
	SCOPE_HOST     Scope = unix.RT_SCOPE_HOST
	SCOPE_NOWHERE  Scope = unix.RT_SCOPE_NOWHERE
)

func (s Scope) String() string {
	switch s {
	case SCOPE_UNIVERSE:
		return "universe"
	case SCOPE_SITE:
		return "site"
	case SCOPE_LINK:
		return "link"
	case SCOPE_HOST:
		return "host"
	case SCOPE_NOWHERE:
		return "nowhere"
	default:
		return "unknown"
	}
}

const (
	RT_FILTER_PROTOCOL uint64 = 1 << (1 + iota)
	RT_FILTER_SCOPE
	RT_FILTER_TYPE
	RT_FILTER_TOS
	RT_FILTER_IIF
	RT_FILTER_OIF
	RT_FILTER_DST
	RT_FILTER_SRC
	RT_FILTER_GW
	RT_FILTER_TABLE
	RT_FILTER_HOPLIMIT
	RT_FILTER_PRIORITY
	RT_FILTER_MARK
	RT_FILTER_MASK
)

const (
	FLAG_ONLINK    NextHopFlag = unix.RTNH_F_ONLINK
	FLAG_PERVASIVE NextHopFlag = unix.RTNH_F_PERVASIVE
)

var testFlags = []flagString{
	{f: FLAG_ONLINK, s: "onlink"},
	{f: FLAG_PERVASIVE, s: "pervasive"},
}

func listFlags(flag int) []string {
	var flags []string
	for _, tf := range testFlags {
		if flag&int(tf.f) != 0 {
			flags = append(flags, tf.s)
		}
	}
	return flags
}

func (r *Route) ListFlags() []string {
	return listFlags(r.Flags)
}

func (n *NexthopInfo) ListFlags() []string {
	return listFlags(n.Flags)
}

type MPLSDestination struct {
	Labels []int
}

func (d *MPLSDestination) Family() int {
	return nl.FAMILY_MPLS
}

func (d *MPLSDestination) Decode(buf []byte) error {
	d.Labels = nl.DecodeMPLSStack(buf)
	return nil
}

func (d *MPLSDestination) Encode() ([]byte, error) {
	return nl.EncodeMPLSStack(d.Labels...), nil
}

func (d *MPLSDestination) String() string {
	s := make([]string, 0, len(d.Labels))
	for _, l := range d.Labels {
		s = append(s, fmt.Sprintf("%d", l))
	}
	return strings.Join(s, "/")
}

func (d *MPLSDestination) Equal(x Destination) bool {
	o, ok := x.(*MPLSDestination)
	if !ok {
		return false
	}
	if d == nil && o == nil {
		return true
	}
	if d == nil || o == nil {
		return false
	}
	if d.Labels == nil && o.Labels == nil {
		return true
	}
	if d.Labels == nil || o.Labels == nil {
		return false
	}
	if len(d.Labels) != len(o.Labels) {
		return false
	}
	for i := range d.Labels {
		if d.Labels[i] != o.Labels[i] {
			return false
		}
	}
	return true
}

type MPLSEncap struct {
	Labels []int
}

func (e *MPLSEncap) Type() int {
	return nl.LWTUNNEL_ENCAP_MPLS
}

func (e *MPLSEncap) Decode(buf []byte) error {
	if len(buf) < 4 {
		return fmt.Errorf("lack of bytes")
	}
	native := nl.NativeEndian()
	l := native.Uint16(buf)
	if len(buf) < int(l) {
		return fmt.Errorf("lack of bytes")
	}
	buf = buf[:l]
	typ := native.Uint16(buf[2:])
	if typ != nl.MPLS_IPTUNNEL_DST {
		return fmt.Errorf("unknown MPLS Encap Type: %d", typ)
	}
	e.Labels = nl.DecodeMPLSStack(buf[4:])
	return nil
}

func (e *MPLSEncap) Encode() ([]byte, error) {
	s := nl.EncodeMPLSStack(e.Labels...)
	native := nl.NativeEndian()
	hdr := make([]byte, 4)
	native.PutUint16(hdr, uint16(len(s)+4))
	native.PutUint16(hdr[2:], nl.MPLS_IPTUNNEL_DST)
	return append(hdr, s...), nil
}

func (e *MPLSEncap) String() string {
	s := make([]string, 0, len(e.Labels))
	for _, l := range e.Labels {
		s = append(s, fmt.Sprintf("%d", l))
	}
	return strings.Join(s, "/")
}

func (e *MPLSEncap) Equal(x Encap) bool {
	o, ok := x.(*MPLSEncap)
	if !ok {
		return false
	}
	if e == nil && o == nil {
		return true
	}
	if e == nil || o == nil {
		return false
	}
	if e.Labels == nil && o.Labels == nil {
		return true
	}
	if e.Labels == nil || o.Labels == nil {
		return false
	}
	if len(e.Labels) != len(o.Labels) {
		return false
	}
	for i := range e.Labels {
		if e.Labels[i] != o.Labels[i] {
			return false
		}
	}
	return true
}

// SEG6 definitions
type SEG6Encap struct {
	Mode     int
	Segments []net.IP
}

func (e *SEG6Encap) Type() int {
	return nl.LWTUNNEL_ENCAP_SEG6
}
func (e *SEG6Encap) Decode(buf []byte) error {
	if len(buf) < 4 {
		return fmt.Errorf("lack of bytes")
	}
	native := nl.NativeEndian()
	// Get Length(l) & Type(typ) : 2 + 2 bytes
	l := native.Uint16(buf)
	if len(buf) < int(l) {
		return fmt.Errorf("lack of bytes")
	}
	buf = buf[:l] // make sure buf size upper limit is Length
	typ := native.Uint16(buf[2:])
	// LWTUNNEL_ENCAP_SEG6 has only one attr type SEG6_IPTUNNEL_SRH
	if typ != nl.SEG6_IPTUNNEL_SRH {
		return fmt.Errorf("unknown SEG6 Type: %d", typ)
	}

	var err error
	e.Mode, e.Segments, err = nl.DecodeSEG6Encap(buf[4:])

	return err
}
func (e *SEG6Encap) Encode() ([]byte, error) {
	s, err := nl.EncodeSEG6Encap(e.Mode, e.Segments)
	native := nl.NativeEndian()
	hdr := make([]byte, 4)
	native.PutUint16(hdr, uint16(len(s)+4))
	native.PutUint16(hdr[2:], nl.SEG6_IPTUNNEL_SRH)
	return append(hdr, s...), err
}
func (e *SEG6Encap) String() string {
	segs := make([]string, 0, len(e.Segments))
	// append segment backwards (from n to 0) since seg#0 is the last segment.
	for i := len(e.Segments); i > 0; i-- {
		segs = append(segs, fmt.Sprintf("%s", e.Segments[i-1]))
	}
	str := fmt.Sprintf("mode %s segs %d [ %s ]", nl.SEG6EncapModeString(e.Mode),
		len(e.Segments), strings.Join(segs, " "))
	return str
}
func (e *SEG6Encap) Equal(x Encap) bool {
	o, ok := x.(*SEG6Encap)
	if !ok {
		return false
	}
	if e == o {
		return true
	}
	if e == nil || o == nil {
		return false
	}
	if e.Mode != o.Mode {
		return false
	}
	if len(e.Segments) != len(o.Segments) {
		return false
	}
	for i := range e.Segments {
		if !e.Segments[i].Equal(o.Segments[i]) {
			return false
		}
	}
	return true
}

// SEG6LocalEncap definitions
type SEG6LocalEncap struct {
	Flags    [nl.SEG6_LOCAL_MAX]bool
	Action   int
	Segments []net.IP // from SRH in seg6_local_lwt
	Table    int      // table id for End.T and End.DT6
	InAddr   net.IP
	In6Addr  net.IP
	Iif      int
	Oif      int
}

func (e *SEG6LocalEncap) Type() int {
	return nl.LWTUNNEL_ENCAP_SEG6_LOCAL
}
func (e *SEG6LocalEncap) Decode(buf []byte) error {
	attrs, err := nl.ParseRouteAttr(buf)
	if err != nil {
		return err
	}
	native := nl.NativeEndian()
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case nl.SEG6_LOCAL_ACTION:
			e.Action = int(native.Uint32(attr.Value[0:4]))
			e.Flags[nl.SEG6_LOCAL_ACTION] = true
		case nl.SEG6_LOCAL_SRH:
			e.Segments, err = nl.DecodeSEG6Srh(attr.Value[:])
			e.Flags[nl.SEG6_LOCAL_SRH] = true
		case nl.SEG6_LOCAL_TABLE:
			e.Table = int(native.Uint32(attr.Value[0:4]))
			e.Flags[nl.SEG6_LOCAL_TABLE] = true
		case nl.SEG6_LOCAL_NH4:
			e.InAddr = net.IP(attr.Value[0:4])
			e.Flags[nl.SEG6_LOCAL_NH4] = true
		case nl.SEG6_LOCAL_NH6:
			e.In6Addr = net.IP(attr.Value[0:16])
			e.Flags[nl.SEG6_LOCAL_NH6] = true
		case nl.SEG6_LOCAL_IIF:
			e.Iif = int(native.Uint32(attr.Value[0:4]))
			e.Flags[nl.SEG6_LOCAL_IIF] = true
		case nl.SEG6_LOCAL_OIF:
			e.Oif = int(native.Uint32(attr.Value[0:4]))
			e.Flags[nl.SEG6_LOCAL_OIF] = true
		}
	}
	return err
}
func (e *SEG6LocalEncap) Encode() ([]byte, error) {
	var err error
	native := nl.NativeEndian()
	res := make([]byte, 8)
	native.PutUint16(res, 8) // length
	native.PutUint16(res[2:], nl.SEG6_LOCAL_ACTION)
	native.PutUint32(res[4:], uint32(e.Action))
	if e.Flags[nl.SEG6_LOCAL_SRH] {
		srh, err := nl.EncodeSEG6Srh(e.Segments)
		if err != nil {
			return nil, err
		}
		attr := make([]byte, 4)
		native.PutUint16(attr, uint16(len(srh)+4))
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_SRH)
		attr = append(attr, srh...)
		res = append(res, attr...)
	}
	if e.Flags[nl.SEG6_LOCAL_TABLE] {
		attr := make([]byte, 8)
		native.PutUint16(attr, 8)
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_TABLE)
		native.PutUint32(attr[4:], uint32(e.Table))
		res = append(res, attr...)
	}
	if e.Flags[nl.SEG6_LOCAL_NH4] {
		attr := make([]byte, 4)
		native.PutUint16(attr, 8)
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_NH4)
		ipv4 := e.InAddr.To4()
		if ipv4 == nil {
			err = fmt.Errorf("SEG6_LOCAL_NH4 has invalid IPv4 address")
			return nil, err
		}
		attr = append(attr, ipv4...)
		res = append(res, attr...)
	}
	if e.Flags[nl.SEG6_LOCAL_NH6] {
		attr := make([]byte, 4)
		native.PutUint16(attr, 20)
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_NH6)
		attr = append(attr, e.In6Addr...)
		res = append(res, attr...)
	}
	if e.Flags[nl.SEG6_LOCAL_IIF] {
		attr := make([]byte, 8)
		native.PutUint16(attr, 8)
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_IIF)
		native.PutUint32(attr[4:], uint32(e.Iif))
		res = append(res, attr...)
	}
	if e.Flags[nl.SEG6_LOCAL_OIF] {
		attr := make([]byte, 8)
		native.PutUint16(attr, 8)
		native.PutUint16(attr[2:], nl.SEG6_LOCAL_OIF)
		native.PutUint32(attr[4:], uint32(e.Oif))
		res = append(res, attr...)
	}
	return res, err
}
func (e *SEG6LocalEncap) String() string {
	strs := make([]string, 0, nl.SEG6_LOCAL_MAX)
	strs = append(strs, fmt.Sprintf("action %s", nl.SEG6LocalActionString(e.Action)))

	if e.Flags[nl.SEG6_LOCAL_TABLE] {
		strs = append(strs, fmt.Sprintf("table %d", e.Table))
	}
	if e.Flags[nl.SEG6_LOCAL_NH4] {
		strs = append(strs, fmt.Sprintf("nh4 %s", e.InAddr))
	}
	if e.Flags[nl.SEG6_LOCAL_NH6] {
		strs = append(strs, fmt.Sprintf("nh6 %s", e.In6Addr))
	}
	if e.Flags[nl.SEG6_LOCAL_IIF] {
		link, err := LinkByIndex(e.Iif)
		if err != nil {
			strs = append(strs, fmt.Sprintf("iif %d", e.Iif))
		} else {
			strs = append(strs, fmt.Sprintf("iif %s", link.Attrs().Name))
		}
	}
	if e.Flags[nl.SEG6_LOCAL_OIF] {
		link, err := LinkByIndex(e.Oif)
		if err != nil {
			strs = append(strs, fmt.Sprintf("oif %d", e.Oif))
		} else {
			strs = append(strs, fmt.Sprintf("oif %s", link.Attrs().Name))
		}
	}
	if e.Flags[nl.SEG6_LOCAL_SRH] {
		segs := make([]string, 0, len(e.Segments))
		//append segment backwards (from n to 0) since seg#0 is the last segment.
		for i := len(e.Segments); i > 0; i-- {
			segs = append(segs, fmt.Sprintf("%s", e.Segments[i-1]))
		}
		strs = append(strs, fmt.Sprintf("segs %d [ %s ]", len(e.Segments), strings.Join(segs, " ")))
	}
	return strings.Join(strs, " ")
}
func (e *SEG6LocalEncap) Equal(x Encap) bool {
	o, ok := x.(*SEG6LocalEncap)
	if !ok {
		return false
	}
	if e == o {
		return true
	}
	if e == nil || o == nil {
		return false
	}
	// compare all arrays first
	for i := range e.Flags {
		if e.Flags[i] != o.Flags[i] {
			return false
		}
	}
	if len(e.Segments) != len(o.Segments) {
		return false
	}
	for i := range e.Segments {
		if !e.Segments[i].Equal(o.Segments[i]) {
			return false
		}
	}
	// compare values
	if !e.InAddr.Equal(o.InAddr) || !e.In6Addr.Equal(o.In6Addr) {
		return false
	}
	if e.Action != o.Action || e.Table != o.Table || e.Iif != o.Iif || e.Oif != o.Oif {
		return false
	}
	return true
}

type Via struct {
	AddrFamily int
	Addr       net.IP
}

func (v *Via) Equal(x Destination) bool {
	o, ok := x.(*Via)
	if !ok {
		return false
	}
	if v.AddrFamily == x.Family() && v.Addr.Equal(o.Addr) {
		return true
	}
	return false
}

func (v *Via) String() string {
	return fmt.Sprintf("Family: %d, Address: %s", v.AddrFamily, v.Addr.String())
}

func (v *Via) Family() int {
	return v.AddrFamily
}

func (v *Via) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, native, uint16(v.AddrFamily))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, native, v.Addr)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *Via) Decode(b []byte) error {
	native := nl.NativeEndian()
	if len(b) < 6 {
		return fmt.Errorf("decoding failed: buffer too small (%d bytes)", len(b))
	}
	v.AddrFamily = int(native.Uint16(b[0:2]))
	if v.AddrFamily == nl.FAMILY_V4 {
		v.Addr = net.IP(b[2:6])
		return nil
	} else if v.AddrFamily == nl.FAMILY_V6 {
		if len(b) < 18 {
			return fmt.Errorf("decoding failed: buffer too small (%d bytes)", len(b))
		}
		v.Addr = net.IP(b[2:])
		return nil
	}
	return fmt.Errorf("decoding failed: address family %d unknown", v.AddrFamily)
}

// RouteAdd will add a route to the system.
// Equivalent to: `ip route add $route`
func RouteAdd(route *Route) error {
	return pkgHandle.RouteAdd(route)
}

// RouteAdd will add a route to the system.
// Equivalent to: `ip route add $route`
func (h *Handle) RouteAdd(route *Route) error {
	flags := unix.NLM_F_CREATE | unix.NLM_F_EXCL | unix.NLM_F_ACK
	req := h.newNetlinkRequest(unix.RTM_NEWROUTE, flags)
	return h.routeHandle(route, req, nl.NewRtMsg())
}

// RouteAppend will append a route to the system.
// Equivalent to: `ip route append $route`
func RouteAppend(route *Route) error {
	return pkgHandle.RouteAppend(route)
}

// RouteAppend will append a route to the system.
// Equivalent to: `ip route append $route`
func (h *Handle) RouteAppend(route *Route) error {
	flags := unix.NLM_F_CREATE | unix.NLM_F_APPEND | unix.NLM_F_ACK
	req := h.newNetlinkRequest(unix.RTM_NEWROUTE, flags)
	return h.routeHandle(route, req, nl.NewRtMsg())
}

// RouteAddEcmp will add a route to the system.
func RouteAddEcmp(route *Route) error {
        return pkgHandle.RouteAddEcmp(route)
}

// RouteAddEcmp will add a route to the system.
func (h *Handle) RouteAddEcmp(route *Route) error {
        flags := unix.NLM_F_CREATE | unix.NLM_F_ACK
        req := h.newNetlinkRequest(unix.RTM_NEWROUTE, flags)
        return h.routeHandle(route, req, nl.NewRtMsg())
}

// RouteReplace will add a route to the system.
// Equivalent to: `ip route replace $route`
func RouteReplace(route *Route) error {
	return pkgHandle.RouteReplace(route)
}

// RouteReplace will add a route to the system.
// Equivalent to: `ip route replace $route`
func (h *Handle) RouteReplace(route *Route) error {
	flags := unix.NLM_F_CREATE | unix.NLM_F_REPLACE | unix.NLM_F_ACK
	req := h.newNetlinkRequest(unix.RTM_NEWROUTE, flags)
	return h.routeHandle(route, req, nl.NewRtMsg())
}

// RouteDel will delete a route from the system.
// Equivalent to: `ip route del $route`
func RouteDel(route *Route) error {
	return pkgHandle.RouteDel(route)
}

// RouteDel will delete a route from the system.
// Equivalent to: `ip route del $route`
func (h *Handle) RouteDel(route *Route) error {
	req := h.newNetlinkRequest(unix.RTM_DELROUTE, unix.NLM_F_ACK)
	return h.routeHandle(route, req, nl.NewRtDelMsg())
}

func (h *Handle) routeHandle(route *Route, req *nl.NetlinkRequest, msg *nl.RtMsg) error {
	if (route.Dst == nil || route.Dst.IP == nil) && route.Src == nil && route.Gw == nil && route.MPLSDst == nil {
		return fmt.Errorf("one of Dst.IP, Src, or Gw must not be nil")
	}

	family := -1
	var rtAttrs []*nl.RtAttr

	if route.Dst != nil && route.Dst.IP != nil {
		dstLen, _ := route.Dst.Mask.Size()
		msg.Dst_len = uint8(dstLen)
		dstFamily := nl.GetIPFamily(route.Dst.IP)
		family = dstFamily
		var dstData []byte
		if dstFamily == FAMILY_V4 {
			dstData = route.Dst.IP.To4()
		} else {
			dstData = route.Dst.IP.To16()
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_DST, dstData))
	} else if route.MPLSDst != nil {
		family = nl.FAMILY_MPLS
		msg.Dst_len = uint8(20)
		msg.Type = unix.RTN_UNICAST
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_DST, nl.EncodeMPLSStack(*route.MPLSDst)))
	}

	if route.NewDst != nil {
		if family != -1 && family != route.NewDst.Family() {
			return fmt.Errorf("new destination and destination are not the same address family")
		}
		buf, err := route.NewDst.Encode()
		if err != nil {
			return err
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_NEWDST, buf))
	}

	if route.Encap != nil {
		buf := make([]byte, 2)
		native.PutUint16(buf, uint16(route.Encap.Type()))
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_ENCAP_TYPE, buf))
		buf, err := route.Encap.Encode()
		if err != nil {
			return err
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_ENCAP, buf))
	}

	if route.Src != nil {
		srcFamily := nl.GetIPFamily(route.Src)
		if family != -1 && family != srcFamily {
			return fmt.Errorf("source and destination ip are not the same IP family")
		}
		family = srcFamily
		var srcData []byte
		if srcFamily == FAMILY_V4 {
			srcData = route.Src.To4()
		} else {
			srcData = route.Src.To16()
		}
		// The commonly used src ip for routes is actually PREFSRC
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_PREFSRC, srcData))
	}

	if route.Gw != nil {
		gwFamily := nl.GetIPFamily(route.Gw)
		if family != -1 && family != gwFamily {
			return fmt.Errorf("gateway, source, and destination ip are not the same IP family")
		}
		family = gwFamily
		var gwData []byte
		if gwFamily == FAMILY_V4 {
			gwData = route.Gw.To4()
		} else {
			gwData = route.Gw.To16()
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_GATEWAY, gwData))
	}

	if route.Via != nil {
		buf, err := route.Via.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode RTA_VIA: %v", err)
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_VIA, buf))
	}

	if len(route.MultiPath) > 0 {
		buf := []byte{}
		for _, nh := range route.MultiPath {
			rtnh := &nl.RtNexthop{
				RtNexthop: unix.RtNexthop{
					Hops:    uint8(nh.Hops),
					Ifindex: int32(nh.LinkIndex),
					Flags:   uint8(nh.Flags),
				},
			}
			children := []nl.NetlinkRequestData{}
			if nh.Gw != nil {
				gwFamily := nl.GetIPFamily(nh.Gw)
				if family != -1 && family != gwFamily {
					return fmt.Errorf("gateway, source, and destination ip are not the same IP family")
				}
				if gwFamily == FAMILY_V4 {
					children = append(children, nl.NewRtAttr(unix.RTA_GATEWAY, []byte(nh.Gw.To4())))
				} else {
					children = append(children, nl.NewRtAttr(unix.RTA_GATEWAY, []byte(nh.Gw.To16())))
				}
			}
			if nh.NewDst != nil {
				if family != -1 && family != nh.NewDst.Family() {
					return fmt.Errorf("new destination and destination are not the same address family")
				}
				buf, err := nh.NewDst.Encode()
				if err != nil {
					return err
				}
				children = append(children, nl.NewRtAttr(unix.RTA_NEWDST, buf))
			}
			if nh.Encap != nil {
				buf := make([]byte, 2)
				native.PutUint16(buf, uint16(nh.Encap.Type()))
				children = append(children, nl.NewRtAttr(unix.RTA_ENCAP_TYPE, buf))
				buf, err := nh.Encap.Encode()
				if err != nil {
					return err
				}
				children = append(children, nl.NewRtAttr(unix.RTA_ENCAP, buf))
			}
			if nh.Via != nil {
				buf, err := nh.Via.Encode()
				if err != nil {
					return err
				}
				children = append(children, nl.NewRtAttr(unix.RTA_VIA, buf))
			}
			rtnh.Children = children
			buf = append(buf, rtnh.Serialize()...)
		}
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_MULTIPATH, buf))
	}

	if route.Table > 0 {
		if route.Table >= 256 {
			msg.Table = unix.RT_TABLE_UNSPEC
			b := make([]byte, 4)
			native.PutUint32(b, uint32(route.Table))
			rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_TABLE, b))
		} else {
			msg.Table = uint8(route.Table)
		}
	}

	if route.Priority > 0 {
		b := make([]byte, 4)
		native.PutUint32(b, uint32(route.Priority))
		rtAttrs = append(rtAttrs, nl.NewRtAttr(unix.RTA_PRIORITY, b))
	}
	if route.Tos > 0 {
		msg.Tos = uint8(route.Tos)
	}
	if route.Protocol > 0 {
		msg.Protocol = uint8(route.Protocol)
	}
	if route.Type > 0 {
		msg.Type = uint8(route.Type)
	}

	var metrics []*nl.RtAttr
	if route.MTU > 0 {
		b := nl.Uint32Attr(uint32(route.MTU))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_MTU, b))
	}
	if route.Window > 0 {
		b := nl.Uint32Attr(uint32(route.Window))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_WINDOW, b))
	}
	if route.Rtt > 0 {
		b := nl.Uint32Attr(uint32(route.Rtt))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_RTT, b))
	}
	if route.RttVar > 0 {
		b := nl.Uint32Attr(uint32(route.RttVar))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_RTTVAR, b))
	}
	if route.Ssthresh > 0 {
		b := nl.Uint32Attr(uint32(route.Ssthresh))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_SSTHRESH, b))
	}
	if route.Cwnd > 0 {
		b := nl.Uint32Attr(uint32(route.Cwnd))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_CWND, b))
	}
	if route.AdvMSS > 0 {
		b := nl.Uint32Attr(uint32(route.AdvMSS))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_ADVMSS, b))
	}
	if route.Reordering > 0 {
		b := nl.Uint32Attr(uint32(route.Reordering))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_REORDERING, b))
	}
	if route.Hoplimit > 0 {
		b := nl.Uint32Attr(uint32(route.Hoplimit))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_HOPLIMIT, b))
	}
	if route.InitCwnd > 0 {
		b := nl.Uint32Attr(uint32(route.InitCwnd))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_INITCWND, b))
	}
	if route.Features > 0 {
		b := nl.Uint32Attr(uint32(route.Features))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_FEATURES, b))
	}
	if route.RtoMin > 0 {
		b := nl.Uint32Attr(uint32(route.RtoMin))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_RTO_MIN, b))
	}
	if route.InitRwnd > 0 {
		b := nl.Uint32Attr(uint32(route.InitRwnd))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_INITRWND, b))
	}
	if route.QuickACK > 0 {
		b := nl.Uint32Attr(uint32(route.QuickACK))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_QUICKACK, b))
	}
	if route.Congctl != "" {
		b := nl.ZeroTerminated(route.Congctl)
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_CC_ALGO, b))
	}
	if route.FastOpenNoCookie > 0 {
		b := nl.Uint32Attr(uint32(route.FastOpenNoCookie))
		metrics = append(metrics, nl.NewRtAttr(unix.RTAX_FASTOPEN_NO_COOKIE, b))
	}

	if metrics != nil {
		attr := nl.NewRtAttr(unix.RTA_METRICS, nil)
		for _, metric := range metrics {
			attr.AddChild(metric)
		}
		rtAttrs = append(rtAttrs, attr)
	}

	msg.Flags = uint32(route.Flags)
	msg.Scope = uint8(route.Scope)
	msg.Family = uint8(family)
	req.AddData(msg)
	for _, attr := range rtAttrs {
		req.AddData(attr)
	}

	var (
		b      = make([]byte, 4)
		native = nl.NativeEndian()
	)
	native.PutUint32(b, uint32(route.LinkIndex))

	req.AddData(nl.NewRtAttr(unix.RTA_OIF, b))

	_, err := req.Execute(unix.NETLINK_ROUTE, 0)
	return err
}

// RouteList gets a list of routes in the system.
// Equivalent to: `ip route show`.
// The list can be filtered by link and ip family.
func RouteList(link Link, family int) ([]Route, error) {
	return pkgHandle.RouteList(link, family)
}

// RouteList gets a list of routes in the system.
// Equivalent to: `ip route show`.
// The list can be filtered by link and ip family.
func (h *Handle) RouteList(link Link, family int) ([]Route, error) {
	var routeFilter *Route
	if link != nil {
		routeFilter = &Route{
			LinkIndex: link.Attrs().Index,
		}
	}
	return h.RouteListFiltered(family, routeFilter, RT_FILTER_OIF)
}

// RouteListFiltered gets a list of routes in the system filtered with specified rules.
// All rules must be defined in RouteFilter struct
func RouteListFiltered(family int, filter *Route, filterMask uint64) ([]Route, error) {
	return pkgHandle.RouteListFiltered(family, filter, filterMask)
}

// RouteListFiltered gets a list of routes in the system filtered with specified rules.
// All rules must be defined in RouteFilter struct
func (h *Handle) RouteListFiltered(family int, filter *Route, filterMask uint64) ([]Route, error) {
	req := h.newNetlinkRequest(unix.RTM_GETROUTE, unix.NLM_F_DUMP)
	infmsg := nl.NewIfInfomsg(family)
	req.AddData(infmsg)

	msgs, err := req.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWROUTE)
	if err != nil {
		return nil, err
	}

	var res []Route
	for _, m := range msgs {
		msg := nl.DeserializeRtMsg(m)
		if msg.Flags&unix.RTM_F_CLONED != 0 {
			// Ignore cloned routes
			continue
		}
		if msg.Table != unix.RT_TABLE_MAIN {
			if filter == nil || filter != nil && filterMask&RT_FILTER_TABLE == 0 {
				// Ignore non-main tables
				continue
			}
		}
		route, err := deserializeRoute(m)
		if err != nil {
			return nil, err
		}
		if filter != nil {
			switch {
			case filterMask&RT_FILTER_TABLE != 0 && filter.Table != unix.RT_TABLE_UNSPEC && route.Table != filter.Table:
				continue
			case filterMask&RT_FILTER_PROTOCOL != 0 && route.Protocol != filter.Protocol:
				continue
			case filterMask&RT_FILTER_SCOPE != 0 && route.Scope != filter.Scope:
				continue
			case filterMask&RT_FILTER_TYPE != 0 && route.Type != filter.Type:
				continue
			case filterMask&RT_FILTER_TOS != 0 && route.Tos != filter.Tos:
				continue
			case filterMask&RT_FILTER_OIF != 0 && route.LinkIndex != filter.LinkIndex:
				continue
			case filterMask&RT_FILTER_IIF != 0 && route.ILinkIndex != filter.ILinkIndex:
				continue
			case filterMask&RT_FILTER_GW != 0 && !route.Gw.Equal(filter.Gw):
				continue
			case filterMask&RT_FILTER_SRC != 0 && !route.Src.Equal(filter.Src):
				continue
			case filterMask&RT_FILTER_DST != 0:
				if filter.MPLSDst == nil || route.MPLSDst == nil || (*filter.MPLSDst) != (*route.MPLSDst) {
					if !ipNetEqual(route.Dst, filter.Dst) {
						continue
					}
				}
			case filterMask&RT_FILTER_HOPLIMIT != 0 && route.Hoplimit != filter.Hoplimit:
				continue
			}
		}
		res = append(res, route)
	}
	return res, nil
}

// deserializeRoute decodes a binary netlink message into a Route struct
func deserializeRoute(m []byte) (Route, error) {
	msg := nl.DeserializeRtMsg(m)
	attrs, err := nl.ParseRouteAttr(m[msg.Len():])
	if err != nil {
		return Route{}, err
	}
	route := Route{
		Scope:    Scope(msg.Scope),
		Protocol: RouteProtocol(int(msg.Protocol)),
		Table:    int(msg.Table),
		Type:     int(msg.Type),
		Tos:      int(msg.Tos),
		Flags:    int(msg.Flags),
	}

	native := nl.NativeEndian()
	var encap, encapType syscall.NetlinkRouteAttr
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case unix.RTA_GATEWAY:
			route.Gw = net.IP(attr.Value)
		case unix.RTA_PREFSRC:
			route.Src = net.IP(attr.Value)
		case unix.RTA_DST:
			if msg.Family == nl.FAMILY_MPLS {
				stack := nl.DecodeMPLSStack(attr.Value)
				if len(stack) == 0 || len(stack) > 1 {
					return route, fmt.Errorf("invalid MPLS RTA_DST")
				}
				route.MPLSDst = &stack[0]
			} else {
				route.Dst = &net.IPNet{
					IP:   attr.Value,
					Mask: net.CIDRMask(int(msg.Dst_len), 8*len(attr.Value)),
				}
			}
		case unix.RTA_OIF:
			route.LinkIndex = int(native.Uint32(attr.Value[0:4]))
		case unix.RTA_IIF:
			route.ILinkIndex = int(native.Uint32(attr.Value[0:4]))
		case unix.RTA_PRIORITY:
			route.Priority = int(native.Uint32(attr.Value[0:4]))
		case unix.RTA_TABLE:
			route.Table = int(native.Uint32(attr.Value[0:4]))
		case unix.RTA_MULTIPATH:
			parseRtNexthop := func(value []byte) (*NexthopInfo, []byte, error) {
				if len(value) < unix.SizeofRtNexthop {
					return nil, nil, fmt.Errorf("lack of bytes")
				}
				nh := nl.DeserializeRtNexthop(value)
				if len(value) < int(nh.RtNexthop.Len) {
					return nil, nil, fmt.Errorf("lack of bytes")
				}
				info := &NexthopInfo{
					LinkIndex: int(nh.RtNexthop.Ifindex),
					Hops:      int(nh.RtNexthop.Hops),
					Flags:     int(nh.RtNexthop.Flags),
				}
				attrs, err := nl.ParseRouteAttr(value[unix.SizeofRtNexthop:int(nh.RtNexthop.Len)])
				if err != nil {
					return nil, nil, err
				}
				var encap, encapType syscall.NetlinkRouteAttr
				for _, attr := range attrs {
					switch attr.Attr.Type {
					case unix.RTA_GATEWAY:
						info.Gw = net.IP(attr.Value)
					case unix.RTA_NEWDST:
						var d Destination
						switch msg.Family {
						case nl.FAMILY_MPLS:
							d = &MPLSDestination{}
						}
						if err := d.Decode(attr.Value); err != nil {
							return nil, nil, err
						}
						info.NewDst = d
					case unix.RTA_ENCAP_TYPE:
						encapType = attr
					case unix.RTA_ENCAP:
						encap = attr
					case unix.RTA_VIA:
						d := &Via{}
						if err := d.Decode(attr.Value); err != nil {
							return nil, nil, err
						}
						info.Via = d
					}
				}

				if len(encap.Value) != 0 && len(encapType.Value) != 0 {
					typ := int(native.Uint16(encapType.Value[0:2]))
					var e Encap
					switch typ {
					case nl.LWTUNNEL_ENCAP_MPLS:
						e = &MPLSEncap{}
						if err := e.Decode(encap.Value); err != nil {
							return nil, nil, err
						}
					}
					info.Encap = e
				}

				return info, value[int(nh.RtNexthop.Len):], nil
			}
			rest := attr.Value
			for len(rest) > 0 {
				info, buf, err := parseRtNexthop(rest)
				if err != nil {
					return route, err
				}
				route.MultiPath = append(route.MultiPath, info)
				rest = buf
			}
		case unix.RTA_NEWDST:
			var d Destination
			switch msg.Family {
			case nl.FAMILY_MPLS:
				d = &MPLSDestination{}
			}
			if err := d.Decode(attr.Value); err != nil {
				return route, err
			}
			route.NewDst = d
		case unix.RTA_VIA:
			v := &Via{}
			if err := v.Decode(attr.Value); err != nil {
				return route, err
			}
			route.Via = v
		case unix.RTA_ENCAP_TYPE:
			encapType = attr
		case unix.RTA_ENCAP:
			encap = attr
		case unix.RTA_METRICS:
			metrics, err := nl.ParseRouteAttr(attr.Value)
			if err != nil {
				return route, err
			}
			for _, metric := range metrics {
				switch metric.Attr.Type {
				case unix.RTAX_MTU:
					route.MTU = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_WINDOW:
					route.Window = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_RTT:
					route.Rtt = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_RTTVAR:
					route.RttVar = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_SSTHRESH:
					route.Ssthresh = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_CWND:
					route.Cwnd = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_ADVMSS:
					route.AdvMSS = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_REORDERING:
					route.Reordering = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_HOPLIMIT:
					route.Hoplimit = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_INITCWND:
					route.InitCwnd = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_FEATURES:
					route.Features = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_RTO_MIN:
					route.RtoMin = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_INITRWND:
					route.InitRwnd = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_QUICKACK:
					route.QuickACK = int(native.Uint32(metric.Value[0:4]))
				case unix.RTAX_CC_ALGO:
					route.Congctl = nl.BytesToString(metric.Value)
				case unix.RTAX_FASTOPEN_NO_COOKIE:
					route.FastOpenNoCookie = int(native.Uint32(metric.Value[0:4]))
				}
			}
		}
	}

	if len(encap.Value) != 0 && len(encapType.Value) != 0 {
		typ := int(native.Uint16(encapType.Value[0:2]))
		var e Encap
		switch typ {
		case nl.LWTUNNEL_ENCAP_MPLS:
			e = &MPLSEncap{}
			if err := e.Decode(encap.Value); err != nil {
				return route, err
			}
		case nl.LWTUNNEL_ENCAP_SEG6:
			e = &SEG6Encap{}
			if err := e.Decode(encap.Value); err != nil {
				return route, err
			}
		case nl.LWTUNNEL_ENCAP_SEG6_LOCAL:
			e = &SEG6LocalEncap{}
			if err := e.Decode(encap.Value); err != nil {
				return route, err
			}
		}
		route.Encap = e
	}

	return route, nil
}

// RouteGetOptions contains a set of options to use with
// RouteGetWithOptions
type RouteGetOptions struct {
	VrfName string
	SrcAddr net.IP
}

// RouteGetWithOptions gets a route to a specific destination from the host system.
// Equivalent to: 'ip route get <> vrf <VrfName>'.
func RouteGetWithOptions(destination net.IP, options *RouteGetOptions) ([]Route, error) {
	return pkgHandle.RouteGetWithOptions(destination, options)
}

// RouteGet gets a route to a specific destination from the host system.
// Equivalent to: 'ip route get'.
func RouteGet(destination net.IP) ([]Route, error) {
	return pkgHandle.RouteGet(destination)
}

// RouteGetWithOptions gets a route to a specific destination from the host system.
// Equivalent to: 'ip route get <> vrf <VrfName>'.
func (h *Handle) RouteGetWithOptions(destination net.IP, options *RouteGetOptions) ([]Route, error) {
	req := h.newNetlinkRequest(unix.RTM_GETROUTE, unix.NLM_F_REQUEST)
	family := nl.GetIPFamily(destination)
	var destinationData []byte
	var bitlen uint8
	if family == FAMILY_V4 {
		destinationData = destination.To4()
		bitlen = 32
	} else {
		destinationData = destination.To16()
		bitlen = 128
	}
	msg := &nl.RtMsg{}
	msg.Family = uint8(family)
	msg.Dst_len = bitlen
	if options != nil && options.SrcAddr != nil {
		msg.Src_len = bitlen
	}
	msg.Flags = unix.RTM_F_LOOKUP_TABLE
	req.AddData(msg)

	rtaDst := nl.NewRtAttr(unix.RTA_DST, destinationData)
	req.AddData(rtaDst)

	if options != nil {
		if options.VrfName != "" {
			link, err := LinkByName(options.VrfName)
			if err != nil {
				return nil, err
			}
			var (
				b      = make([]byte, 4)
				native = nl.NativeEndian()
			)
			native.PutUint32(b, uint32(link.Attrs().Index))

			req.AddData(nl.NewRtAttr(unix.RTA_OIF, b))
		}

		if options.SrcAddr != nil {
			var srcAddr []byte
			if family == FAMILY_V4 {
				srcAddr = options.SrcAddr.To4()
			} else {
				srcAddr = options.SrcAddr.To16()
			}

			req.AddData(nl.NewRtAttr(unix.RTA_SRC, srcAddr))
		}
	}

	msgs, err := req.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWROUTE)
	if err != nil {
		return nil, err
	}

	var res []Route
	for _, m := range msgs {
		route, err := deserializeRoute(m)
		if err != nil {
			return nil, err
		}
		res = append(res, route)
	}
	return res, nil
}

// RouteGet gets a route to a specific destination from the host system.
// Equivalent to: 'ip route get'.
func (h *Handle) RouteGet(destination net.IP) ([]Route, error) {
	return h.RouteGetWithOptions(destination, nil)
}

// RouteSubscribe takes a chan down which notifications will be sent
// when routes are added or deleted. Close the 'done' chan to stop subscription.
func RouteSubscribe(ch chan<- RouteUpdate, done <-chan struct{}) error {
	return routeSubscribeAt(netns.None(), netns.None(), ch, done, nil, false)
}

// RouteSubscribeAt works like RouteSubscribe plus it allows the caller
// to choose the network namespace in which to subscribe (ns).
func RouteSubscribeAt(ns netns.NsHandle, ch chan<- RouteUpdate, done <-chan struct{}) error {
	return routeSubscribeAt(ns, netns.None(), ch, done, nil, false)
}

// RouteSubscribeOptions contains a set of options to use with
// RouteSubscribeWithOptions.
type RouteSubscribeOptions struct {
	Namespace     *netns.NsHandle
	ErrorCallback func(error)
	ListExisting  bool
}

// RouteSubscribeWithOptions work like RouteSubscribe but enable to
// provide additional options to modify the behavior. Currently, the
// namespace can be provided as well as an error callback.
func RouteSubscribeWithOptions(ch chan<- RouteUpdate, done <-chan struct{}, options RouteSubscribeOptions) error {
	if options.Namespace == nil {
		none := netns.None()
		options.Namespace = &none
	}
	return routeSubscribeAt(*options.Namespace, netns.None(), ch, done, options.ErrorCallback, options.ListExisting)
}

func routeSubscribeAt(newNs, curNs netns.NsHandle, ch chan<- RouteUpdate, done <-chan struct{}, cberr func(error), listExisting bool) error {
	s, err := nl.SubscribeAt(newNs, curNs, unix.NETLINK_ROUTE, unix.RTNLGRP_IPV4_ROUTE, unix.RTNLGRP_IPV6_ROUTE)
	if err != nil {
		return err
	}
	if done != nil {
		go func() {
			<-done
			s.Close()
		}()
	}
	if listExisting {
		req := pkgHandle.newNetlinkRequest(unix.RTM_GETROUTE,
			unix.NLM_F_DUMP)
		infmsg := nl.NewIfInfomsg(unix.AF_UNSPEC)
		req.AddData(infmsg)
		if err := s.Send(req); err != nil {
			return err
		}
	}
	go func() {
		defer close(ch)
		for {
			msgs, from, err := s.Receive()
			if err != nil {
				if cberr != nil {
					cberr(err)
				}
				return
			}
			if from.Pid != nl.PidKernel {
				if cberr != nil {
					cberr(fmt.Errorf("Wrong sender portid %d, expected %d", from.Pid, nl.PidKernel))
				}
				continue
			}
			for _, m := range msgs {
				if m.Header.Type == unix.NLMSG_DONE {
					continue
				}
				if m.Header.Type == unix.NLMSG_ERROR {
					native := nl.NativeEndian()
					error := int32(native.Uint32(m.Data[0:4]))
					if error == 0 {
						continue
					}
					if cberr != nil {
						cberr(syscall.Errno(-error))
					}
					return
				}
				route, err := deserializeRoute(m.Data)
				if err != nil {
					if cberr != nil {
						cberr(err)
					}
					return
				}
				ch <- RouteUpdate{Type: m.Header.Type, Route: route}
			}
		}
	}()

	return nil
}

func (p RouteProtocol) String() string {
	switch int(p) {
	case unix.RTPROT_BABEL:
		return "babel"
	case unix.RTPROT_BGP:
		return "bgp"
	case unix.RTPROT_BIRD:
		return "bird"
	case unix.RTPROT_BOOT:
		return "boot"
	case unix.RTPROT_DHCP:
		return "dhcp"
	case unix.RTPROT_DNROUTED:
		return "dnrouted"
	case unix.RTPROT_EIGRP:
		return "eigrp"
	case unix.RTPROT_GATED:
		return "gated"
	case unix.RTPROT_ISIS:
		return "isis"
	//case unix.RTPROT_KEEPALIVED:
	//	return "keepalived"
	case unix.RTPROT_KERNEL:
		return "kernel"
	case unix.RTPROT_MROUTED:
		return "mrouted"
	case unix.RTPROT_MRT:
		return "mrt"
	case unix.RTPROT_NTK:
		return "ntk"
	case unix.RTPROT_OSPF:
		return "ospf"
	case unix.RTPROT_RA:
		return "ra"
	case unix.RTPROT_REDIRECT:
		return "redirect"
	case unix.RTPROT_RIP:
		return "rip"
	case unix.RTPROT_STATIC:
		return "static"
	case unix.RTPROT_UNSPEC:
		return "unspec"
	case unix.RTPROT_XORP:
		return "xorp"
	case unix.RTPROT_ZEBRA:
		return "zebra"
	default:
		return strconv.Itoa(int(p))
	}
}
