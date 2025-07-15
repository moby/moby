// Package resolvconf provides utility code to query and update DNS configuration in /etc/resolv.conf
package resolvconf

import "github.com/docker/docker/daemon/libnetwork/resolvconf"

const (
	// Deprecated: use [resolvconf.IP] instead.
	IP = resolvconf.IP
	// Deprecated: use [resolvconf.IPv4] instead.
	IPv4 = resolvconf.IPv4
	// Deprecated: use [resolvconf.IPv6] instead.
	IPv6 = resolvconf.IPv6
)

// File contains the resolv.conf content and its hash
//
// Deprecated: use [resolvconf.File] instead.
type File = resolvconf.File

// Path returns the path to the resolv.conf file that libnetwork should use.
//
// Deprecated: use [resolvconf.Path] instead.
var Path = resolvconf.Path

// Get returns the contents of /etc/resolv.conf and its hash
//
// Deprecated: use [resolvconf.Get] instead.
var Get = resolvconf.Get

// GetSpecific returns the contents of the user specified resolv.conf file and its hash
//
// Deprecated: use [resolvconf.GetSpecific] instead.
var GetSpecific = resolvconf.GetSpecific

// FilterResolvDNS cleans up the config in resolvConf.  It has two main jobs:
//  1. It looks for localhost (127.*|::1) entries in the provided
//     resolv.conf, removing local nameserver entries, and, if the resulting
//     cleaned config has no defined nameservers left, adds default DNS entries
//  2. Given the caller provides the enable/disable state of IPv6, the filter
//     code will remove all IPv6 nameservers if it is not enabled for containers
//
// Deprecated: use [resolvconf.FilterResolvDNS] instead.
var FilterResolvDNS = resolvconf.FilterResolvDNS

// GetNameservers returns nameservers (if any) listed in /etc/resolv.conf
//
// Deprecated: use [resolvconf.GetNameservers] instead.
var GetNameservers = resolvconf.GetNameservers

// GetNameserversAsPrefix returns nameservers (if any) listed in
// /etc/resolv.conf as CIDR blocks (e.g., "1.2.3.4/32")
//
// Deprecated: use [resolvconf.GetNameserversAsPrefix] instead.
var GetNameserversAsPrefix = resolvconf.GetNameserversAsPrefix

// GetSearchDomains returns search domains (if any) listed in /etc/resolv.conf
// If more than one search line is encountered, only the contents of the last
// one is returned.
//
// Deprecated: use [resolvconf.GetSearchDomains] instead.
var GetSearchDomains = resolvconf.GetSearchDomains

// GetOptions returns options (if any) listed in /etc/resolv.conf
// If more than one options line is encountered, only the contents of the last
// one is returned.
//
// Deprecated: use [resolvconf.GetOptions] instead.
var GetOptions = resolvconf.GetOptions

// Build generates and writes a configuration file to path containing a nameserver
// entry for every element in nameservers, a "search" entry for every element in
// dnsSearch, and an "options" entry for every element in dnsOptions. It returns
// a File containing the generated content and its (sha256) hash.
//
// Note that the resolv.conf file is written, but the hash file is not.
//
// Deprecated: use [resolvconf.Build] instead.
var Build = resolvconf.Build
