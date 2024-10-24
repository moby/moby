package networking

import (
	"net"
	"syscall"
	"testing"

	"github.com/docker/docker/internal/nlwrap"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
)

// FindHostAddr returns the address of the host machine that would be used to
// reach external hosts. family can be either unix.AF_INET, or unix.AF_INET6.
func FindHostAddr(t *testing.T, family int) string {
	t.Helper()

	var dst string
	// These are IPv4 and IPv6 addresses of Cloudflare's DNS servers. These
	// don't really matter - we just need to find a route to some external
	// server.
	if family == netlink.FAMILY_V4 {
		dst = "1.1.1.1"
	} else if family == netlink.FAMILY_V6 {
		dst = "2606:4700:4700::1111"
	} else {
		t.Fatalf("unknown address family %d", family)
	}

	routes, err := netlink.RouteGetWithOptions(net.ParseIP(dst), &netlink.RouteGetOptions{FIBMatch: true})
	if err != nil && err == syscall.ENETUNREACH {
		t.Skipf("no %s route available", familyName(family))
	}
	assert.NilError(t, err, "looking up route to %s: %v", dst, err)
	assert.Assert(t, len(routes) > 0, "no route found for %s", dst)

	l, err := netlink.LinkByIndex(routes[0].LinkIndex)
	assert.NilError(t, err, "looking up link %d: %v", routes[0].LinkIndex, err)

	addrs, err := nlwrap.AddrList(l, family)
	assert.NilError(t, err, "looking up addresses for link %s: %v", l.Attrs().Name, err)
	assert.Assert(t, len(addrs) > 0, "no addresses found for link %s", l.Attrs().Name)

	return addrs[0].IP.String()
}

func familyName(family int) string {
	if family == netlink.FAMILY_V4 {
		return "IPv4"
	}
	return "IPv6"
}
