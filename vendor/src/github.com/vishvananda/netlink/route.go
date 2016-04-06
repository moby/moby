package netlink

import (
	"fmt"
	"net"
	"syscall"
)

// Scope is an enum representing a route scope.
type Scope uint8

const (
	SCOPE_UNIVERSE Scope = syscall.RT_SCOPE_UNIVERSE
	SCOPE_SITE     Scope = syscall.RT_SCOPE_SITE
	SCOPE_LINK     Scope = syscall.RT_SCOPE_LINK
	SCOPE_HOST     Scope = syscall.RT_SCOPE_HOST
	SCOPE_NOWHERE  Scope = syscall.RT_SCOPE_NOWHERE
)

type NextHopFlag int

const (
	FLAG_ONLINK    NextHopFlag = syscall.RTNH_F_ONLINK
	FLAG_PERVASIVE NextHopFlag = syscall.RTNH_F_PERVASIVE
)

// Route represents a netlink route.
type Route struct {
	LinkIndex  int
	ILinkIndex int
	Scope      Scope
	Dst        *net.IPNet
	Src        net.IP
	Gw         net.IP
	Protocol   int
	Priority   int
	Table      int
	Type       int
	Tos        int
	Flags      int
}

func (r Route) String() string {
	return fmt.Sprintf("{Ifindex: %d Dst: %s Src: %s Gw: %s Flags: %s}", r.LinkIndex, r.Dst,
		r.Src, r.Gw, r.ListFlags())
}

func (r *Route) SetFlag(flag NextHopFlag) {
	r.Flags |= int(flag)
}

func (r *Route) ClearFlag(flag NextHopFlag) {
	r.Flags &^= int(flag)
}

type flagString struct {
	f NextHopFlag
	s string
}

var testFlags = []flagString{
	flagString{f: FLAG_ONLINK, s: "onlink"},
	flagString{f: FLAG_PERVASIVE, s: "pervasive"},
}

func (r *Route) ListFlags() []string {
	var flags []string
	for _, tf := range testFlags {
		if r.Flags&int(tf.f) != 0 {
			flags = append(flags, tf.s)
		}
	}
	return flags
}

// RouteUpdate is sent when a route changes - type is RTM_NEWROUTE or RTM_DELROUTE
type RouteUpdate struct {
	Type uint16
	Route
}
