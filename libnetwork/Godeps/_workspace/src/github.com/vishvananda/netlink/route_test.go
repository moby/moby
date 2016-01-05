package netlink

import (
	"net"
	"syscall"
	"testing"
	"time"
)

func TestRouteAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	dstIP := net.IPv4(192, 168, 0, 42)
	routeToDstIP, err := RouteGet(dstIP)
	if err != nil {
		t.Fatal(err)
	}

	if len(routeToDstIP) == 0 {
		t.Fatal("Default route not present")
	}
	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}

}

func TestRouteAddIncomplete(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	route := Route{LinkIndex: link.Attrs().Index}
	if err := RouteAdd(&route); err == nil {
		t.Fatal("Adding incomplete route should fail")
	}
}

func expectRouteUpdate(ch <-chan RouteUpdate, t uint16, dst net.IP) bool {
	for {
		timeout := time.After(time.Minute)
		select {
		case update := <-ch:
			if update.Type == t && update.Route.Dst.IP.Equal(dst) {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

func TestRouteSubscribe(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := RouteSubscribe(ch, done); err != nil {
		t.Fatal(err)
	}

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, syscall.RTM_NEWROUTE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}
	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	if !expectRouteUpdate(ch, syscall.RTM_DELROUTE, dst.IP) {
		t.Fatal("Del update not received as expected")
	}
}

func TestRouteExtraFields(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(1, 1, 1, 1),
		Mask: net.CIDRMask(32, 32),
	}

	src := net.IPv4(127, 3, 3, 3)
	route := Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Src:       src,
		Scope:     syscall.RT_SCOPE_LINK,
		Priority:  13,
		Table:     syscall.RT_TABLE_MAIN,
		Type:      syscall.RTN_UNICAST,
		Tos:       14,
	}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteListFiltered(FAMILY_V4, &Route{
		Dst:   dst,
		Src:   src,
		Scope: syscall.RT_SCOPE_LINK,
		Table: syscall.RT_TABLE_MAIN,
		Type:  syscall.RTN_UNICAST,
		Tos:   14,
	}, RT_FILTER_DST|RT_FILTER_SRC|RT_FILTER_SCOPE|RT_FILTER_TABLE|RT_FILTER_TYPE|RT_FILTER_TOS)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if routes[0].Scope != syscall.RT_SCOPE_LINK {
		t.Fatal("Invalid Scope. Route not added properly")
	}
	if routes[0].Priority != 13 {
		t.Fatal("Invalid Priority. Route not added properly")
	}
	if routes[0].Table != syscall.RT_TABLE_MAIN {
		t.Fatal("Invalid Scope. Route not added properly")
	}
	if routes[0].Type != syscall.RTN_UNICAST {
		t.Fatal("Invalid Type. Route not added properly")
	}
	if routes[0].Tos != 14 {
		t.Fatal("Invalid Tos. Route not added properly")
	}
}
