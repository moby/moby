package etchosts

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Record Structure for a single host record
type Record struct {
	Hosts string
	IP    string
}

// WriteTo writes record to file and returns bytes written or error
func (r Record) WriteTo(w io.Writer) (int64, error) {
	n, err := fmt.Fprintf(w, "%s\t%s\n", r.IP, r.Hosts)
	return int64(n), err
}

var (
	// Default hosts config records slice
	defaultContentIPv4 = []Record{
		{Hosts: "localhost", IP: "127.0.0.1"},
	}
	defaultContentIPv6 = []Record{
		{Hosts: "localhost ip6-localhost ip6-loopback", IP: "::1"},
		{Hosts: "ip6-localnet", IP: "fe00::0"},
		{Hosts: "ip6-mcastprefix", IP: "ff00::0"},
		{Hosts: "ip6-allnodes", IP: "ff02::1"},
		{Hosts: "ip6-allrouters", IP: "ff02::2"},
	}

	// A cache of path level locks for synchronizing /etc/hosts
	// updates on a file level
	pathMap = make(map[string]*sync.Mutex)

	// A package level mutex to synchronize the cache itself
	pathMutex sync.Mutex
)

func pathLock(path string) func() {
	pathMutex.Lock()
	defer pathMutex.Unlock()

	pl, ok := pathMap[path]
	if !ok {
		pl = &sync.Mutex{}
		pathMap[path] = pl
	}

	pl.Lock()
	return func() {
		pl.Unlock()
	}
}

// Drop drops the path string from the path cache
func Drop(path string) {
	pathMutex.Lock()
	defer pathMutex.Unlock()

	delete(pathMap, path)
}

// Build function
// path is path to host file string required
// IP, hostname, and domainname set main record leave empty for no master record
// extraContent is an array of extra host records.
func Build(path, IP, hostname, domainname string, extraContent []Record) error {
	return build(path, IP, hostname, domainname, defaultContentIPv4, defaultContentIPv6, extraContent)
}

// BuildNoIPv6 is the same as Build, but will not include IPv6 entries.
func BuildNoIPv6(path, IP, hostname, domainname string, extraContent []Record) error {
	if isIPv6(IP) {
		IP = ""
	}

	var ipv4ExtraContent []Record
	for _, rec := range extraContent {
		if !isIPv6(rec.IP) {
			ipv4ExtraContent = append(ipv4ExtraContent, rec)
		}
	}

	return build(path, IP, hostname, domainname, defaultContentIPv4, ipv4ExtraContent)
}

func isIPv6(s string) bool {
	addr, err := netip.ParseAddr(s)
	return err == nil && addr.Is6()
}

func build(path, IP, hostname, domainname string, contents ...[]Record) error {
	defer pathLock(path)()

	buf := bytes.NewBuffer(nil)
	if IP != "" {
		// set main record
		var mainRec Record
		mainRec.IP = IP
		// User might have provided a FQDN in hostname or split it across hostname
		// and domainname.  We want the FQDN and the bare hostname.
		fqdn := hostname
		if domainname != "" {
			fqdn += "." + domainname
		}
		mainRec.Hosts = fqdn

		if hostName, _, ok := strings.Cut(fqdn, "."); ok {
			mainRec.Hosts += " " + hostName
		}
		if _, err := mainRec.WriteTo(buf); err != nil {
			return err
		}
	}

	// Write content from function arguments
	for _, content := range contents {
		for _, c := range content {
			if _, err := c.WriteTo(buf); err != nil {
				return err
			}
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Add adds an arbitrary number of Records to an already existing /etc/hosts file
func Add(path string, recs []Record) error {
	defer pathLock(path)()

	if len(recs) == 0 {
		return nil
	}

	b, err := mergeRecords(path, recs)
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o644)
}

func mergeRecords(path string, recs []Record) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	content := bytes.NewBuffer(nil)

	if _, err := content.ReadFrom(f); err != nil {
		return nil, err
	}

	for _, r := range recs {
		if _, err := r.WriteTo(content); err != nil {
			return nil, err
		}
	}

	return content.Bytes(), nil
}

// Delete deletes an arbitrary number of Records already existing in /etc/hosts file
func Delete(path string, recs []Record) error {
	defer pathLock(path)()

	if len(recs) == 0 {
		return nil
	}
	old, err := os.Open(path)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	s := bufio.NewScanner(old)
	eol := []byte{'\n'}
loop:
	for s.Scan() {
		b := s.Bytes()
		if len(b) == 0 {
			continue
		}

		if b[0] == '#' {
			buf.Write(b)
			buf.Write(eol)
			continue
		}
		for _, r := range recs {
			if bytes.HasSuffix(b, []byte("\t"+r.Hosts)) {
				continue loop
			}
		}
		buf.Write(b)
		buf.Write(eol)
	}
	old.Close()
	if err := s.Err(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Update all IP addresses where hostname matches.
// path is path to host file
// IP is new IP address
// hostname is hostname to search for to replace IP
func Update(path, IP, hostname string) error {
	defer pathLock(path)()

	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(fmt.Sprintf("(\\S*)(\\t%s)(\\s|\\.)", regexp.QuoteMeta(hostname)))
	return os.WriteFile(path, re.ReplaceAll(old, []byte(IP+"$2"+"$3")), 0o644)
}
