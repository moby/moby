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
	IP    netip.Addr
}

// WriteTo writes record to file and returns bytes written or error
func (r Record) WriteTo(w io.Writer) (int64, error) {
	n, err := fmt.Fprintf(w, "%s\t%s\n", r.IP, r.Hosts)
	return int64(n), err
}

var (
	// Default hosts config records slice
	defaultContentIPv4 = []Record{
		{Hosts: "localhost", IP: netip.MustParseAddr("127.0.0.1")},
	}
	defaultContentIPv6 = []Record{
		{Hosts: "localhost ip6-localhost ip6-loopback", IP: netip.IPv6Loopback()},
		{Hosts: "ip6-localnet", IP: netip.MustParseAddr("fe00::")},
		{Hosts: "ip6-mcastprefix", IP: netip.MustParseAddr("ff00::")},
		{Hosts: "ip6-allnodes", IP: netip.MustParseAddr("ff02::1")},
		{Hosts: "ip6-allrouters", IP: netip.MustParseAddr("ff02::2")},
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
// extraContent is an array of extra host records.
func Build(path string, extraContent []Record) error {
	return build(path, defaultContentIPv4, defaultContentIPv6, extraContent)
}

// BuildNoIPv6 is the same as Build, but will not include IPv6 entries.
func BuildNoIPv6(path string, extraContent []Record) error {
	var ipv4ExtraContent []Record
	for _, rec := range extraContent {
		if !rec.IP.Is6() {
			ipv4ExtraContent = append(ipv4ExtraContent, rec)
		}
	}
	return build(path, defaultContentIPv4, ipv4ExtraContent)
}

func build(path string, contents ...[]Record) error {
	defer pathLock(path)()

	buf := bytes.NewBuffer(nil)

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
	if len(recs) == 0 {
		return nil
	}

	defer pathLock(path)()

	content := bytes.NewBuffer(nil)
	for _, r := range recs {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(content.Bytes())
	_ = f.Close()
	return err
}

// Delete deletes Records from /etc/hosts.
// The hostnames must be an exact match (if the user has modified the record,
// it won't be deleted). The address, parsed as a netip.Addr must also match
// the value in recs.
func Delete(path string, recs []Record) error {
	if len(recs) == 0 {
		return nil
	}
	defer pathLock(path)()
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer

	s := bufio.NewScanner(f)
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
			if before, found := strings.CutSuffix(string(b), "\t"+r.Hosts); found {
				if addr, err := netip.ParseAddr(strings.TrimSpace(before)); err == nil && addr == r.IP {
					continue loop
				}
			}
		}
		buf.Write(b)
		buf.Write(eol)
	}
	if err := s.Err(); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err = f.WriteAt(buf.Bytes(), 0)
	return err
}

// Update all IP addresses where hostname matches.
// path is path to host file
// IP is new IP address
// hostname is hostname to search for to replace IP
func Update(path, IP, hostname string) error {
	re, err := regexp.Compile(fmt.Sprintf(`(\S*)(\t%s)(\s|\.)`, regexp.QuoteMeta(hostname)))
	if err != nil {
		return err
	}
	defer pathLock(path)()

	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, re.ReplaceAll(old, []byte(IP+"$2"+"$3")), 0o644)
}
