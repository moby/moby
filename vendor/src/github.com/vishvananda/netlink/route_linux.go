package netlink

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink/nl"
)

// RtAttr is shared so it is in netlink_linux.go

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
)

// RouteAdd will add a route to the system.
// Equivalent to: `ip route add $route`
func RouteAdd(route *Route) error {
	req := nl.NewNetlinkRequest(syscall.RTM_NEWROUTE, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)
	return routeHandle(route, req, nl.NewRtMsg())
}

// RouteDel will delete a route from the system.
// Equivalent to: `ip route del $route`
func RouteDel(route *Route) error {
	req := nl.NewNetlinkRequest(syscall.RTM_DELROUTE, syscall.NLM_F_ACK)
	return routeHandle(route, req, nl.NewRtDelMsg())
}

func routeHandle(route *Route, req *nl.NetlinkRequest, msg *nl.RtMsg) error {
	if (route.Dst == nil || route.Dst.IP == nil) && route.Src == nil && route.Gw == nil {
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
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_DST, dstData))
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
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_PREFSRC, srcData))
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
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_GATEWAY, gwData))
	}

	if route.Table > 0 {
		if route.Table >= 256 {
			msg.Table = syscall.RT_TABLE_UNSPEC
			b := make([]byte, 4)
			native.PutUint32(b, uint32(route.Table))
			rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_TABLE, b))
		} else {
			msg.Table = uint8(route.Table)
		}
	}

	if route.Priority > 0 {
		b := make([]byte, 4)
		native.PutUint32(b, uint32(route.Priority))
		rtAttrs = append(rtAttrs, nl.NewRtAttr(syscall.RTA_PRIORITY, b))
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

	req.AddData(nl.NewRtAttr(syscall.RTA_OIF, b))

	_, err := req.Execute(syscall.NETLINK_ROUTE, 0)
	return err
}

// RouteList gets a list of routes in the system.
// Equivalent to: `ip route show`.
// The list can be filtered by link and ip family.
func RouteList(link Link, family int) ([]Route, error) {
	var routeFilter *Route
	if link != nil {
		routeFilter = &Route{
			LinkIndex: link.Attrs().Index,
		}
	}
	return RouteListFiltered(family, routeFilter, RT_FILTER_OIF)
}

// RouteListFiltered gets a list of routes in the system filtered with specified rules.
// All rules must be defined in RouteFilter struct
func RouteListFiltered(family int, filter *Route, filterMask uint64) ([]Route, error) {
	req := nl.NewNetlinkRequest(syscall.RTM_GETROUTE, syscall.NLM_F_DUMP)
	infmsg := nl.NewIfInfomsg(family)
	req.AddData(infmsg)

	msgs, err := req.Execute(syscall.NETLINK_ROUTE, syscall.RTM_NEWROUTE)
	if err != nil {
		return nil, err
	}

	var res []Route
	for _, m := range msgs {
		msg := nl.DeserializeRtMsg(m)
		if msg.Flags&syscall.RTM_F_CLONED != 0 {
			// Ignore cloned routes
			continue
		}
		if msg.Table != syscall.RT_TABLE_MAIN {
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
			case filterMask&RT_FILTER_TABLE != 0 && route.Table != filter.Table:
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
			case filterMask&RT_FILTER_DST != 0 && filter.Dst != nil:
				if route.Dst == nil {
					continue
				}
				aMaskLen, aMaskBits := route.Dst.Mask.Size()
				bMaskLen, bMaskBits := filter.Dst.Mask.Size()
				if !(route.Dst.IP.Equal(filter.Dst.IP) && aMaskLen == bMaskLen && aMaskBits == bMaskBits) {
					continue
				}
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
		Protocol: int(msg.Protocol),
		Table:    int(msg.Table),
		Type:     int(msg.Type),
		Tos:      int(msg.Tos),
		Flags:    int(msg.Flags),
	}

	native := nl.NativeEndian()
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case syscall.RTA_GATEWAY:
			route.Gw = net.IP(attr.Value)
		case syscall.RTA_PREFSRC:
			route.Src = net.IP(attr.Value)
		case syscall.RTA_DST:
			route.Dst = &net.IPNet{
				IP:   attr.Value,
				Mask: net.CIDRMask(int(msg.Dst_len), 8*len(attr.Value)),
			}
		case syscall.RTA_OIF:
			route.LinkIndex = int(native.Uint32(attr.Value[0:4]))
		case syscall.RTA_IIF:
			route.ILinkIndex = int(native.Uint32(attr.Value[0:4]))
		case syscall.RTA_PRIORITY:
			route.Priority = int(native.Uint32(attr.Value[0:4]))
		case syscall.RTA_TABLE:
			route.Table = int(native.Uint32(attr.Value[0:4]))
		}
	}
	return route, nil
}

// RouteGet gets a route to a specific destination from the host system.
// Equivalent to: 'ip route get'.
func RouteGet(destination net.IP) ([]Route, error) {
	req := nl.NewNetlinkRequest(syscall.RTM_GETROUTE, syscall.NLM_F_REQUEST)
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
	req.AddData(msg)

	rtaDst := nl.NewRtAttr(syscall.RTA_DST, destinationData)
	req.AddData(rtaDst)

	msgs, err := req.Execute(syscall.NETLINK_ROUTE, syscall.RTM_NEWROUTE)
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

// RouteSubscribe takes a chan down which notifications will be sent
// when routes are added or deleted. Close the 'done' chan to stop subscription.
func RouteSubscribe(ch chan<- RouteUpdate, done <-chan struct{}) error {
	s, err := nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_IPV4_ROUTE, syscall.RTNLGRP_IPV6_ROUTE)
	if err != nil {
		return err
	}
	if done != nil {
		go func() {
			<-done
			s.Close()
		}()
	}
	go func() {
		defer close(ch)
		for {
			msgs, err := s.Receive()
			if err != nil {
				return
			}
			for _, m := range msgs {
				route, err := deserializeRoute(m.Data)
				if err != nil {
					return
				}
				ch <- RouteUpdate{Type: m.Header.Type, Route: route}
			}
		}
	}()

	return nil
}
