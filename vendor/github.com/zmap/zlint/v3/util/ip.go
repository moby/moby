/*
 * ZLint Copyright 2021 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// contains helper functions for ip address lints

package util

import (
	"fmt"
	"net"
)

type subnetCategory int

const (
	privateUse subnetCategory = iota
	sharedAddressSpace
	benchmarking
	documentation
	reserved
	protocolAssignment
	as112
	amt
	orchidV2
	_ // deprecated: lisp
	thisHostOnThisNetwork
	translatableAddress6to4
	translatableAddress4to6
	dummyAddress
	portControlProtocolAnycast
	traversalUsingRelaysAroundNATAnycast
	nat64DNS64Discovery
	limitedBroadcast
	discardOnly
	teredo
	uniqueLocal
	linkLocalUnicast
	ianaReservedForFutureUse
	ianaReservedMulticast
)

var reservedNetworks []*net.IPNet

// IsIANAReserved checks IP validity as per IANA reserved IPs
//      IPv4
//      https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
//      https://www.iana.org/assignments/ipv4-address-space/ipv4-address-space.xml
//      IPv6
//      https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry.xhtml
//      https://www.iana.org/assignments/ipv6-address-space/ipv6-address-space.xhtml
func IsIANAReserved(ip net.IP) bool {
	if !ip.IsGlobalUnicast() {
		return true
	}

	for _, network := range reservedNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// IntersectsIANAReserved checks if a CIDR intersects any IANA reserved CIDRs
func IntersectsIANAReserved(net net.IPNet) bool {
	if !net.IP.IsGlobalUnicast() {
		return true
	}
	for _, reserved := range reservedNetworks {
		if reserved.Contains(net.IP) || net.Contains(reserved.IP) {
			return true
		}
	}
	return false
}

func init() {
	var networks = map[subnetCategory][]string{
		privateUse:                           {"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		sharedAddressSpace:                   {"100.64.0.0/10"},
		benchmarking:                         {"198.18.0.0/15", "2001:2::/48"},
		documentation:                        {"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"},
		reserved:                             {"240.0.0.0/4", "0400::/6", "0800::/5", "1000::/4", "4000::/3", "6000::/3", "8000::/3", "a000::/3", "c000::/3", "e000::/4", "f000::/5", "f800::/6", "fe00::/9"}, // https://www.iana.org/assignments/ipv6-address-space/ipv6-address-space.xhtml
		protocolAssignment:                   {"192.0.0.0/24", "2001::/23"},                                                                                                                                   // 192.0.0.0/24 contains 192.0.0.0/29 - IPv4 Service Continuity Prefix
		as112:                                {"192.31.196.0/24", "192.175.48.0/24", "2001:4:112::/48", "2620:4f:8000::/48"},
		amt:                                  {"192.52.193.0/24", "2001:3::/32"},
		orchidV2:                             {"2001:20::/28"},
		thisHostOnThisNetwork:                {"0.0.0.0/8"},
		translatableAddress4to6:              {"2002::/16"},
		translatableAddress6to4:              {"64:ff9b::/96", "64:ff9b:1::/48"},
		dummyAddress:                         {"192.0.0.8/32"},
		portControlProtocolAnycast:           {"192.0.0.9/32", "2001:1::1/128"},
		traversalUsingRelaysAroundNATAnycast: {"192.0.0.10/32", "2001:1::2/128"},
		nat64DNS64Discovery:                  {"192.0.0.170/32", "192.0.0.171/32"},
		limitedBroadcast:                     {"255.255.255.255/32"},
		discardOnly:                          {"100::/64"},
		teredo:                               {"2001::/32"},
		uniqueLocal:                          {"fc00::/7"},
		linkLocalUnicast:                     {"fe80::/10", "169.254.0.0/16"}, // this range is covered by 	ip.IsLinkLocalUnicast(), which is in turn called by  net.IP.IsGlobalUnicast(ip)
		ianaReservedForFutureUse:             {"255.0.0.0/8", "254.0.0.0/8", "253.0.0.0/8", "252.0.0.0/8", "251.0.0.0/8", "250.0.0.0/8", "249.0.0.0/8", "248.0.0.0/8", "247.0.0.0/8", "246.0.0.0/8", "245.0.0.0/8", "244.0.0.0/8", "243.0.0.0/8", "242.0.0.0/8", "241.0.0.0/8", "240.0.0.0/8"},
		ianaReservedMulticast:                {"239.0.0.0/8", "238.0.0.0/8", "237.0.0.0/8", "236.0.0.0/8", "235.0.0.0/8", "234.0.0.0/8", "233.0.0.0/8", "232.0.0.0/8", "231.0.0.0/8", "230.0.0.0/8", "229.0.0.0/8", "228.0.0.0/8", "227.0.0.0/8", "226.0.0.0/8", "225.0.0.0/8", "224.0.0.0/8", "ff00::/8"}, // this range is covered by ip.IsMulticast() call, which is in turn called by  net.IP.IsGlobalUnicast(ip)
	}

	for _, netList := range networks {
		for _, network := range netList {
			var ipNet *net.IPNet
			var err error

			if _, ipNet, err = net.ParseCIDR(network); err != nil {
				panic(fmt.Sprintf("unexpected internal network value provided: %s", err.Error()))
			}
			reservedNetworks = append(reservedNetworks, ipNet)
		}
	}
}
