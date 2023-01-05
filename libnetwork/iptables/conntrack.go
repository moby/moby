//go:build linux
// +build linux

package iptables

import (
	"errors"
	"net"
	"syscall"

	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var (
	// ErrConntrackNotConfigurable means that conntrack module is not loaded or does not have the netlink module loaded
	ErrConntrackNotConfigurable = errors.New("conntrack is not available")
)

// IsConntrackProgrammable returns true if the handle supports the NETLINK_NETFILTER and the base modules are loaded
func IsConntrackProgrammable(nlh *netlink.Handle) bool {
	return nlh.SupportsNetlinkFamily(syscall.NETLINK_NETFILTER)
}

// DeleteConntrackEntries deletes all the conntrack connections on the host for the specified IP
// Returns the number of flows deleted for IPv4, IPv6 else error
func DeleteConntrackEntries(nlh *netlink.Handle, ipv4List []net.IP, ipv6List []net.IP) (uint, uint, error) {
	if !IsConntrackProgrammable(nlh) {
		return 0, 0, ErrConntrackNotConfigurable
	}

	var totalIPv4FlowPurged uint
	for _, ipAddress := range ipv4List {
		flowPurged, err := purgeConntrackState(nlh, syscall.AF_INET, ipAddress)
		if err != nil {
			logrus.Warnf("Failed to delete conntrack state for %s: %v", ipAddress, err)
			continue
		}
		totalIPv4FlowPurged += flowPurged
	}

	var totalIPv6FlowPurged uint
	for _, ipAddress := range ipv6List {
		flowPurged, err := purgeConntrackState(nlh, syscall.AF_INET6, ipAddress)
		if err != nil {
			logrus.Warnf("Failed to delete conntrack state for %s: %v", ipAddress, err)
			continue
		}
		totalIPv6FlowPurged += flowPurged
	}

	logrus.Debugf("DeleteConntrackEntries purged ipv4:%d, ipv6:%d", totalIPv4FlowPurged, totalIPv6FlowPurged)
	return totalIPv4FlowPurged, totalIPv6FlowPurged, nil
}

func DeleteConntrackEntriesByPort(nlh *netlink.Handle, proto types.Protocol, ports []uint16) error {
	if !IsConntrackProgrammable(nlh) {
		return ErrConntrackNotConfigurable
	}

	var totalIPv4FlowPurged uint
	var totalIPv6FlowPurged uint

	for _, port := range ports {
		filter := &netlink.ConntrackFilter{}
		if err := filter.AddProtocol(uint8(proto)); err != nil {
			logrus.Warnf("Failed to delete conntrack state for %s port %d: %v", proto.String(), port, err)
			continue
		}
		if err := filter.AddPort(netlink.ConntrackOrigDstPort, port); err != nil {
			logrus.Warnf("Failed to delete conntrack state for %s port %d: %v", proto.String(), port, err)
			continue
		}

		v4FlowPurged, err := nlh.ConntrackDeleteFilter(netlink.ConntrackTable, syscall.AF_INET, filter)
		if err != nil {
			logrus.Warnf("Failed to delete conntrack state for IPv4 %s port %d: %v", proto.String(), port, err)
		}
		totalIPv4FlowPurged += v4FlowPurged

		v6FlowPurged, err := nlh.ConntrackDeleteFilter(netlink.ConntrackTable, syscall.AF_INET6, filter)
		if err != nil {
			logrus.Warnf("Failed to delete conntrack state for IPv6 %s port %d: %v", proto.String(), port, err)
		}
		totalIPv6FlowPurged += v6FlowPurged
	}

	logrus.Debugf("DeleteConntrackEntriesByPort for %s ports purged ipv4:%d, ipv6:%d", proto.String(), totalIPv4FlowPurged, totalIPv6FlowPurged)
	return nil
}

func purgeConntrackState(nlh *netlink.Handle, family netlink.InetFamily, ipAddress net.IP) (uint, error) {
	filter := &netlink.ConntrackFilter{}
	// NOTE: doing the flush using the ipAddress is safe because today there cannot be multiple networks with the same subnet
	// so it will not be possible to flush flows that are of other containers
	if err := filter.AddIP(netlink.ConntrackNatAnyIP, ipAddress); err != nil {
		return 0, err
	}
	return nlh.ConntrackDeleteFilter(netlink.ConntrackTable, family, filter)
}
