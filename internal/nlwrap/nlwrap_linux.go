// Package nlwrap wraps vishvandanda/netlink functions that may return EINTR.
//
// A Handle instantiated using [NewHandle] or [NewHandleAt] can be used in place
// of a netlink.Handle, it's a wrapper that replaces methods that need to be
// wrapped. Functions that use the package handle need to be called as "nlwrap.X"
// instead of "netlink.X".
//
// The wrapped functions currently return EINTR when NLM_F_DUMP_INTR flagged
// in a netlink response, meaning something changed during the dump so results
// may be incomplete or inconsistent.
//
// At present, the possibly incomplete/inconsistent results are not returned
// by netlink functions along with the EINTR. So, it's not possible to do
// anything but retry. After maxAttempts the EINTR will be returned to the
// caller.
package nlwrap

import (
	"context"
	"errors"

	"github.com/containerd/log"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// Arbitrary limit on max attempts at netlink calls if they are repeatedly interrupted.
const maxAttempts = 5

type Handle struct {
	*netlink.Handle
}

func NewHandle(nlFamilies ...int) (Handle, error) {
	nlh, err := netlink.NewHandle(nlFamilies...)
	if err != nil {
		return Handle{}, err
	}
	return Handle{nlh}, nil
}

func NewHandleAt(ns netns.NsHandle, nlFamilies ...int) (Handle, error) {
	nlh, err := netlink.NewHandleAt(ns, nlFamilies...)
	if err != nil {
		return Handle{}, err
	}
	return Handle{nlh}, nil
}

func (h Handle) Close() {
	if h.Handle != nil {
		h.Handle.Close()
	}
}

func retryOnIntr(f func() error) {
	for attempt := 0; attempt < maxAttempts; attempt += 1 {
		if err := f(); !errors.Is(err, unix.EINTR) {
			return
		}
	}
	log.G(context.TODO()).Infof("netlink call interrupted after %d attempts", maxAttempts)
}

// AddrList calls nlh.LinkList, retrying if necessary.
func (nlh Handle) AddrList(link netlink.Link, family int) (addrs []netlink.Addr, err error) {
	retryOnIntr(func() error {
		addrs, err = nlh.Handle.AddrList(link, family) //nolint:forbidigo
		return err
	})
	return addrs, err
}

// AddrList calls netlink.LinkList, retrying if necessary.
func AddrList(link netlink.Link, family int) (addrs []netlink.Addr, err error) {
	retryOnIntr(func() error {
		addrs, err = netlink.AddrList(link, family) //nolint:forbidigo
		return err
	})
	return addrs, err
}

// ConntrackDeleteFilters calls nlh.ConntrackDeleteFilters, retrying if necessary.
func (nlh Handle) ConntrackDeleteFilters(
	table netlink.ConntrackTableType,
	family netlink.InetFamily,
	filters ...netlink.CustomConntrackFilter,
) (matched uint, err error) {
	retryOnIntr(func() error {
		matched, err = nlh.Handle.ConntrackDeleteFilters(table, family, filters...) //nolint:forbidigo
		return err
	})
	return matched, err
}

// ConntrackTableList calls netlink.ConntrackTableList, retrying if necessary.
func ConntrackTableList(
	table netlink.ConntrackTableType,
	family netlink.InetFamily,
) (flows []*netlink.ConntrackFlow, err error) {
	retryOnIntr(func() error {
		flows, err = netlink.ConntrackTableList(table, family) //nolint:forbidigo
		return err
	})
	return flows, err
}

// LinkByName calls nlh.LinkByName, retrying if necessary. The netlink function
// doesn't normally ask the kernel for a dump of links. But, on an old kernel, it
// will do as a fallback and that dump may get inconsistent results.
func (nlh Handle) LinkByName(name string) (link netlink.Link, err error) {
	retryOnIntr(func() error {
		link, err = nlh.Handle.LinkByName(name) //nolint:forbidigo
		return err
	})
	return link, err
}

// LinkByName calls netlink.LinkByName, retrying if necessary. The netlink
// function doesn't normally ask the kernel for a dump of links. But, on an old
// kernel, it will do as a fallback and that dump may get inconsistent results.
func LinkByName(name string) (link netlink.Link, err error) {
	retryOnIntr(func() error {
		link, err = netlink.LinkByName(name) //nolint:forbidigo
		return err
	})
	return link, err
}

// LinkList calls nlh.LinkList, retrying if necessary.
func (nlh Handle) LinkList() (links []netlink.Link, err error) {
	retryOnIntr(func() error {
		links, err = nlh.Handle.LinkList() //nolint:forbidigo
		return err
	})
	return links, err
}

// LinkList calls netlink.LinkList, retrying if necessary.
func LinkList() (links []netlink.Link, err error) {
	retryOnIntr(func() error {
		links, err = netlink.LinkList() //nolint:forbidigo
		return err
	})
	return links, err
}

// RouteList calls nlh.RouteList, retrying if necessary.
func (nlh Handle) RouteList(link netlink.Link, family int) (routes []netlink.Route, err error) {
	retryOnIntr(func() error {
		routes, err = nlh.Handle.RouteList(link, family) //nolint:forbidigo
		return err
	})
	return routes, err
}

func (nlh Handle) XfrmPolicyList(family int) (policies []netlink.XfrmPolicy, err error) {
	retryOnIntr(func() error {
		policies, err = nlh.Handle.XfrmPolicyList(family) //nolint:forbidigo
		return err
	})
	return policies, err
}

func (nlh Handle) XfrmStateList(family int) (states []netlink.XfrmState, err error) {
	retryOnIntr(func() error {
		states, err = nlh.Handle.XfrmStateList(family) //nolint:forbidigo
		return err
	})
	return states, err
}
