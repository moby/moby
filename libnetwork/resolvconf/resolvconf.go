// Package resolvconf provides utility code to query and update DNS configuration in /etc/resolv.conf
package resolvconf

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	// defaultPath is the default path to the resolv.conf that contains information to resolve DNS. See Path().
	defaultPath = "/etc/resolv.conf"
	// alternatePath is a path different from defaultPath, that may be used to resolve DNS. See Path().
	alternatePath = "/run/systemd/resolve/resolv.conf"
)

// constants for the IP address type
const (
	IP = iota // IPv4 and IPv6
	IPv4
	IPv6
)

var (
	detectSystemdResolvConfOnce sync.Once
	pathAfterSystemdDetection   = defaultPath
)

// Path returns the path to the resolv.conf file that libnetwork should use.
//
// When /etc/resolv.conf contains 127.0.0.53 as the only nameserver, then
// it is assumed systemd-resolved manages DNS. Because inside the container 127.0.0.53
// is not a valid DNS server, Path() returns /run/systemd/resolve/resolv.conf
// which is the resolv.conf that systemd-resolved generates and manages.
// Otherwise Path() returns /etc/resolv.conf.
//
// Errors are silenced as they will inevitably resurface at future open/read calls.
//
// More information at https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html#/etc/resolv.conf
func Path() string {
	detectSystemdResolvConfOnce.Do(func() {
		candidateResolvConf, err := os.ReadFile(defaultPath)
		if err != nil {
			// silencing error as it will resurface at next calls trying to read defaultPath
			return
		}
		ns := GetNameservers(candidateResolvConf, IP)
		if len(ns) == 1 && ns[0] == "127.0.0.53" {
			pathAfterSystemdDetection = alternatePath
			logrus.Infof("detected 127.0.0.53 nameserver, assuming systemd-resolved, so using resolv.conf: %s", alternatePath)
		}
	})
	return pathAfterSystemdDetection
}

const (
	// ipLocalhost is a regex pattern for IPv4 or IPv6 loopback range.
	ipLocalhost  = `((127\.([0-9]{1,3}\.){2}[0-9]{1,3})|(::1)$)`
	ipv4NumBlock = `(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`
	ipv4Address  = `(` + ipv4NumBlock + `\.){3}` + ipv4NumBlock

	// This is not an IPv6 address verifier as it will accept a super-set of IPv6, and also
	// will *not match* IPv4-Embedded IPv6 Addresses (RFC6052), but that and other variants
	// -- e.g. other link-local types -- either won't work in containers or are unnecessary.
	// For readability and sufficiency for Docker purposes this seemed more reasonable than a
	// 1000+ character regexp with exact and complete IPv6 validation
	ipv6Address = `([0-9A-Fa-f]{0,4}:){2,7}([0-9A-Fa-f]{0,4})(%\w+)?`
)

var (
	// Note: the default IPv4 & IPv6 resolvers are set to Google's Public DNS
	defaultIPv4Dns = []string{"nameserver 8.8.8.8", "nameserver 8.8.4.4"}
	defaultIPv6Dns = []string{"nameserver 2001:4860:4860::8888", "nameserver 2001:4860:4860::8844"}

	localhostNSRegexp = regexp.MustCompile(`(?m)^nameserver\s+` + ipLocalhost + `\s*\n*`)
	nsIPv6Regexp      = regexp.MustCompile(`(?m)^nameserver\s+` + ipv6Address + `\s*\n*`)
	nsRegexp          = regexp.MustCompile(`^\s*nameserver\s*((` + ipv4Address + `)|(` + ipv6Address + `))\s*$`)
	nsIPv6Regexpmatch = regexp.MustCompile(`^\s*nameserver\s*((` + ipv6Address + `))\s*$`)
	nsIPv4Regexpmatch = regexp.MustCompile(`^\s*nameserver\s*((` + ipv4Address + `))\s*$`)
	searchRegexp      = regexp.MustCompile(`^\s*search\s*(([^\s]+\s*)*)$`)
	optionsRegexp     = regexp.MustCompile(`^\s*options\s*(([^\s]+\s*)*)$`)
)

var lastModified struct {
	sync.Mutex
	sha256   string
	contents []byte
}

// File contains the resolv.conf content and its hash
type File struct {
	Content []byte
	Hash    string
}

// Get returns the contents of /etc/resolv.conf and its hash
func Get() (*File, error) {
	return GetSpecific(Path())
}

// GetSpecific returns the contents of the user specified resolv.conf file and its hash
func GetSpecific(path string) (*File, error) {
	resolv, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash, err := hashData(bytes.NewReader(resolv))
	if err != nil {
		return nil, err
	}
	return &File{Content: resolv, Hash: hash}, nil
}

// GetIfChanged retrieves the host /etc/resolv.conf file, checks against the last hash
// and, if modified since last check, returns the bytes and new hash.
// This feature is used by the resolv.conf updater for containers
func GetIfChanged() (*File, error) {
	lastModified.Lock()
	defer lastModified.Unlock()

	resolv, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	newHash, err := hashData(bytes.NewReader(resolv))
	if err != nil {
		return nil, err
	}
	if lastModified.sha256 != newHash {
		lastModified.sha256 = newHash
		lastModified.contents = resolv
		return &File{Content: resolv, Hash: newHash}, nil
	}
	// nothing changed, so return no data
	return nil, nil
}

// GetLastModified retrieves the last used contents and hash of the host resolv.conf.
// Used by containers updating on restart
func GetLastModified() *File {
	lastModified.Lock()
	defer lastModified.Unlock()

	return &File{Content: lastModified.contents, Hash: lastModified.sha256}
}

// FilterResolvDNS cleans up the config in resolvConf.  It has two main jobs:
//  1. It looks for localhost (127.*|::1) entries in the provided
//     resolv.conf, removing local nameserver entries, and, if the resulting
//     cleaned config has no defined nameservers left, adds default DNS entries
//  2. Given the caller provides the enable/disable state of IPv6, the filter
//     code will remove all IPv6 nameservers if it is not enabled for containers
func FilterResolvDNS(resolvConf []byte, ipv6Enabled bool) (*File, error) {
	cleanedResolvConf := localhostNSRegexp.ReplaceAll(resolvConf, []byte{})
	// if IPv6 is not enabled, also clean out any IPv6 address nameserver
	if !ipv6Enabled {
		cleanedResolvConf = nsIPv6Regexp.ReplaceAll(cleanedResolvConf, []byte{})
	}
	// if the resulting resolvConf has no more nameservers defined, add appropriate
	// default DNS servers for IPv4 and (optionally) IPv6
	if len(GetNameservers(cleanedResolvConf, IP)) == 0 {
		logrus.Infof("No non-localhost DNS nameservers are left in resolv.conf. Using default external servers: %v", defaultIPv4Dns)
		dns := defaultIPv4Dns
		if ipv6Enabled {
			logrus.Infof("IPv6 enabled; Adding default IPv6 external servers: %v", defaultIPv6Dns)
			dns = append(dns, defaultIPv6Dns...)
		}
		cleanedResolvConf = append(cleanedResolvConf, []byte("\n"+strings.Join(dns, "\n"))...)
	}
	hash, err := hashData(bytes.NewReader(cleanedResolvConf))
	if err != nil {
		return nil, err
	}
	return &File{Content: cleanedResolvConf, Hash: hash}, nil
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
func GetNameservers(resolvConf []byte, kind int) []string {
	nameservers := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		var ns [][]byte
		if kind == IP {
			ns = nsRegexp.FindSubmatch(line)
		} else if kind == IPv4 {
			ns = nsIPv4Regexpmatch.FindSubmatch(line)
		} else if kind == IPv6 {
			ns = nsIPv6Regexpmatch.FindSubmatch(line)
		}
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
	for _, nameserver := range GetNameservers(resolvConf, IP) {
		var address string
		// If IPv6, strip zone if present
		if strings.Contains(nameserver, ":") {
			address = strings.Split(nameserver, "%")[0] + "/128"
		} else {
			address = nameserver + "/32"
		}
		nameservers = append(nameservers, address)
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

// GetOptions returns options (if any) listed in /etc/resolv.conf
// If more than one options line is encountered, only the contents of the last
// one is returned.
func GetOptions(resolvConf []byte) []string {
	options := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		match := optionsRegexp.FindSubmatch(line)
		if match == nil {
			continue
		}
		options = strings.Fields(string(match[1]))
	}
	return options
}

// Build writes a configuration file to path containing a "nameserver" entry
// for every element in dns, a "search" entry for every element in
// dnsSearch, and an "options" entry for every element in dnsOptions.
func Build(path string, dns, dnsSearch, dnsOptions []string) (*File, error) {
	content := bytes.NewBuffer(nil)
	if len(dnsSearch) > 0 {
		if searchString := strings.Join(dnsSearch, " "); strings.Trim(searchString, " ") != "." {
			if _, err := content.WriteString("search " + searchString + "\n"); err != nil {
				return nil, err
			}
		}
	}
	for _, dns := range dns {
		if _, err := content.WriteString("nameserver " + dns + "\n"); err != nil {
			return nil, err
		}
	}
	if len(dnsOptions) > 0 {
		if optsString := strings.Join(dnsOptions, " "); strings.Trim(optsString, " ") != "" {
			if _, err := content.WriteString("options " + optsString + "\n"); err != nil {
				return nil, err
			}
		}
	}

	hash, err := hashData(bytes.NewReader(content.Bytes()))
	if err != nil {
		return nil, err
	}

	return &File{Content: content.Bytes(), Hash: hash}, os.WriteFile(path, content.Bytes(), 0644)
}
