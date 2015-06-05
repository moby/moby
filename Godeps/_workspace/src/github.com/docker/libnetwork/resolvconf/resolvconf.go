// Package resolvconf provides utility code to query and update DNS configuration in /etc/resolv.conf
package resolvconf

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/libnetwork/resolvconf/dns"
)

var (
	// Note: the default IPv4 & IPv6 resolvers are set to Google's Public DNS
	defaultIPv4Dns = []string{"nameserver 8.8.8.8", "nameserver 8.8.4.4"}
	defaultIPv6Dns = []string{"nameserver 2001:4860:4860::8888", "nameserver 2001:4860:4860::8844"}
	ipv4NumBlock   = `(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`
	ipv4Address    = `(` + ipv4NumBlock + `\.){3}` + ipv4NumBlock
	// This is not an IPv6 address verifier as it will accept a super-set of IPv6, and also
	// will *not match* IPv4-Embedded IPv6 Addresses (RFC6052), but that and other variants
	// -- e.g. other link-local types -- either won't work in containers or are unnecessary.
	// For readability and sufficiency for Docker purposes this seemed more reasonable than a
	// 1000+ character regexp with exact and complete IPv6 validation
	ipv6Address = `([0-9A-Fa-f]{0,4}:){2,7}([0-9A-Fa-f]{0,4})`

	localhostNSRegexp = regexp.MustCompile(`(?m)^nameserver\s+` + dns.IPLocalhost + `\s*\n*`)
	nsIPv6Regexp      = regexp.MustCompile(`(?m)^nameserver\s+` + ipv6Address + `\s*\n*`)
	nsRegexp          = regexp.MustCompile(`^\s*nameserver\s*((` + ipv4Address + `)|(` + ipv6Address + `))\s*$`)
	searchRegexp      = regexp.MustCompile(`^\s*search\s*(([^\s]+\s*)*)$`)
)

var lastModified struct {
	sync.Mutex
	sha256   string
	contents []byte
}

// Get returns the contents of /etc/resolv.conf
func Get() ([]byte, error) {
	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	return resolv, nil
}

// GetIfChanged retrieves the host /etc/resolv.conf file, checks against the last hash
// and, if modified since last check, returns the bytes and new hash.
// This feature is used by the resolv.conf updater for containers
func GetIfChanged() ([]byte, string, error) {
	lastModified.Lock()
	defer lastModified.Unlock()

	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, "", err
	}
	newHash, err := ioutils.HashData(bytes.NewReader(resolv))
	if err != nil {
		return nil, "", err
	}
	if lastModified.sha256 != newHash {
		lastModified.sha256 = newHash
		lastModified.contents = resolv
		return resolv, newHash, nil
	}
	// nothing changed, so return no data
	return nil, "", nil
}

// GetLastModified retrieves the last used contents and hash of the host resolv.conf.
// Used by containers updating on restart
func GetLastModified() ([]byte, string) {
	lastModified.Lock()
	defer lastModified.Unlock()

	return lastModified.contents, lastModified.sha256
}

// FilterResolvDNS cleans up the config in resolvConf.  It has two main jobs:
// 1. It looks for localhost (127.*|::1) entries in the provided
//    resolv.conf, removing local nameserver entries, and, if the resulting
//    cleaned config has no defined nameservers left, adds default DNS entries
// 2. Given the caller provides the enable/disable state of IPv6, the filter
//    code will remove all IPv6 nameservers if it is not enabled for containers
//
// It returns a boolean to notify the caller if changes were made at all
func FilterResolvDNS(resolvConf []byte, ipv6Enabled bool) ([]byte, bool) {
	changed := false
	cleanedResolvConf := localhostNSRegexp.ReplaceAll(resolvConf, []byte{})
	// if IPv6 is not enabled, also clean out any IPv6 address nameserver
	if !ipv6Enabled {
		cleanedResolvConf = nsIPv6Regexp.ReplaceAll(cleanedResolvConf, []byte{})
	}
	// if the resulting resolvConf has no more nameservers defined, add appropriate
	// default DNS servers for IPv4 and (optionally) IPv6
	if len(GetNameservers(cleanedResolvConf)) == 0 {
		logrus.Infof("No non-localhost DNS nameservers are left in resolv.conf. Using default external servers : %v", defaultIPv4Dns)
		dns := defaultIPv4Dns
		if ipv6Enabled {
			logrus.Infof("IPv6 enabled; Adding default IPv6 external servers : %v", defaultIPv6Dns)
			dns = append(dns, defaultIPv6Dns...)
		}
		cleanedResolvConf = append(cleanedResolvConf, []byte("\n"+strings.Join(dns, "\n"))...)
	}
	if !bytes.Equal(resolvConf, cleanedResolvConf) {
		changed = true
	}
	return cleanedResolvConf, changed
}

// getLines parses input into lines and strips away comments.
func getLines(input []byte, commentMarker []byte) [][]byte {
	lines := bytes.Split(input, []byte("\n"))
	var output [][]byte
	for _, currentLine := range lines {
		var commentIndex = bytes.Index(currentLine, commentMarker)
		if commentIndex == -1 {
			output = append(output, currentLine)
		} else {
			output = append(output, currentLine[:commentIndex])
		}
	}
	return output
}

// GetNameservers returns nameservers (if any) listed in /etc/resolv.conf
func GetNameservers(resolvConf []byte) []string {
	nameservers := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		var ns = nsRegexp.FindSubmatch(line)
		if len(ns) > 0 {
			nameservers = append(nameservers, string(ns[1]))
		}
	}
	return nameservers
}

// GetNameserversAsCIDR returns nameservers (if any) listed in
// /etc/resolv.conf as CIDR blocks (e.g., "1.2.3.4/32")
// This function's output is intended for net.ParseCIDR
func GetNameserversAsCIDR(resolvConf []byte) []string {
	nameservers := []string{}
	for _, nameserver := range GetNameservers(resolvConf) {
		nameservers = append(nameservers, nameserver+"/32")
	}
	return nameservers
}

// GetSearchDomains returns search domains (if any) listed in /etc/resolv.conf
// If more than one search line is encountered, only the contents of the last
// one is returned.
func GetSearchDomains(resolvConf []byte) []string {
	domains := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		match := searchRegexp.FindSubmatch(line)
		if match == nil {
			continue
		}
		domains = strings.Fields(string(match[1]))
	}
	return domains
}

// Build writes a configuration file to path containing a "nameserver" entry
// for every element in dns, and a "search" entry for every element in
// dnsSearch.
func Build(path string, dns, dnsSearch []string) error {
	content := bytes.NewBuffer(nil)
	for _, dns := range dns {
		if _, err := content.WriteString("nameserver " + dns + "\n"); err != nil {
			return err
		}
	}
	if len(dnsSearch) > 0 {
		if searchString := strings.Join(dnsSearch, " "); strings.Trim(searchString, " ") != "." {
			if _, err := content.WriteString("search " + searchString + "\n"); err != nil {
				return err
			}
		}
	}

	return ioutil.WriteFile(path, content.Bytes(), 0644)
}
